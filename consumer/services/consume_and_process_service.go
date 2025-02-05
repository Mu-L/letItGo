package services

import (
	"container/heap"
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/Sumit189/letItGo/common/models"
	"github.com/Sumit189/letItGo/common/repository"
	"github.com/aws/aws-msk-iam-sasl-signer-go/signer"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	workerCount             = 1
	consumerGroupID         = "schedule_processor_group"
	kafkaVersion            = "2.1.0"
	kafkaTopic              = "scheduled_tasks"
	kafkaMinBytes           = 10e3 // 10KB
	kafkaMaxBytes           = 10e6 // 10MB
	kafkaCommitInterval     = time.Second
	processChanBuffer       = 100000 // Adjust buffer size as needed
	shutdownTimeout         = 30 * time.Second
	scheduleDispatchTimeout = 1 * time.Second
	cacheWindow             = 5 * time.Minute
)

// ScheduleHeap is a min-heap based on NextRunTime
type ScheduleHeap []models.Scheduler

func (h ScheduleHeap) Len() int           { return len(h) }
func (h ScheduleHeap) Less(i, j int) bool { return h[i].NextRunTime.Before(*h[j].NextRunTime) }
func (h ScheduleHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *ScheduleHeap) Push(x interface{}) {
	*h = append(*h, x.(models.Scheduler))
}

func (h *ScheduleHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type MSKAccessTokenProvider struct {
}

func (m *MSKAccessTokenProvider) Token() (*sarama.AccessToken, error) {
	token, _, err := signer.GenerateAuthToken(context.TODO(), "ap-south-1")
	return &sarama.AccessToken{Token: token}, err
}

func initKafkaReader() (sarama.ConsumerGroup, error) {
	var kafkaBrokers = []string{os.Getenv("KAFKA_BROKER")}
	config := sarama.NewConfig()
	config.Version = sarama.V2_1_0_0
	config.Net.SASL.Enable = true
	config.Net.SASL.Mechanism = sarama.SASLTypeOAuth
	config.Net.SASL.TokenProvider = &MSKAccessTokenProvider{}
	config.Net.TLS.Enable = true
	config.Net.TLS.Config = &tls.Config{InsecureSkipVerify: false}
	config.Consumer.Return.Errors = true

	consumer, err := sarama.NewConsumerGroup(kafkaBrokers, consumerGroupID, config)
	if err != nil {
		return nil, err
	}
	return consumer, nil
}

func ConsumeAndProcess(ctx context.Context) {
	log.Println("Starting Consumer Service...")

	// Initialize Kafka consumer
	consumer, err := initKafkaReader()
	if err != nil {
		log.Fatalf("Failed to initialize Kafka consumer: %v", err)
	}
	defer func() {
		if err := consumer.Close(); err != nil {
			log.Printf("Error closing Kafka consumer: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := &sync.WaitGroup{}

	// Initialize the schedule heap
	scheduleHeap := &ScheduleHeap{}
	heap.Init(scheduleHeap)
	var heapMutex sync.Mutex
	heapCond := sync.NewCond(&heapMutex)

	// Channel to send due schedules for processing
	processChan := make(chan models.Scheduler, processChanBuffer)

	// Start the scheduler goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		scheduler(ctx, scheduleHeap, &heapMutex, heapCond, processChan)
	}()

	// Start consumer goroutines
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			consume(ctx, consumer, workerID, scheduleHeap, &heapMutex, heapCond)
		}(i)
	}

	// Start processor goroutines
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			processWorker(ctx, processChan, workerID)
		}(i)
	}

	// Handle graceful shutdown
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
	<-sigchan
	log.Println("Shutdown signal received")

	// Initiate shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	cancel()

	// Signal the scheduler to exit by broadcasting to the condition variable
	heapMutex.Lock()
	heapCond.Broadcast()
	heapMutex.Unlock()

	// Close the process channel to signal processors to exit after draining
	close(processChan)

	// Wait for all goroutines to finish or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Consumer Service stopped gracefully")
	case <-shutdownCtx.Done():
		log.Println("Shutdown timed out. Forcing exit.")
	}
}

func consume(ctx context.Context, consumer sarama.ConsumerGroup, workerID int, scheduleHeap *ScheduleHeap, heapMutex *sync.Mutex, heapCond *sync.Cond) {
	handler := &consumerGroupHandler{
		ctx:          ctx,
		workerID:     workerID,
		scheduleHeap: scheduleHeap,
		heapMutex:    heapMutex,
		heapCond:     heapCond,
	}
	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d: Context canceled. Exiting consume.", workerID)
			return
		default:
			if err := consumer.Consume(ctx, []string{kafkaTopic}, handler); err != nil {
				log.Printf("Worker %d: Error consuming messages: %v", workerID, err)
				time.Sleep(time.Second) // Backoff on error
			}
		}
	}
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler
type consumerGroupHandler struct {
	ctx          context.Context
	workerID     int
	scheduleHeap *ScheduleHeap
	heapMutex    *sync.Mutex
	heapCond     *sync.Cond
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *consumerGroupHandler) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	for msg := range claim.Messages() {
		var schedule models.Scheduler
		err := json.Unmarshal(msg.Value, &schedule)
		if err != nil {
			log.Printf("Worker %d: Error unmarshaling schedule ID %s: %v", h.workerID, string(msg.Key), err)
			continue
		}

		h.heapMutex.Lock()
		heap.Push(h.scheduleHeap, schedule)
		h.heapCond.Signal() // Notify the scheduler that a new schedule has been added
		h.heapMutex.Unlock()

		// Mark the message as processed
		session.MarkMessage(msg, "")
	}
	return nil
}

// scheduler manages the schedule heap and dispatches due schedules for processing
func scheduler(ctx context.Context, scheduleHeap *ScheduleHeap, heapMutex *sync.Mutex, heapCond *sync.Cond, processChan chan<- models.Scheduler) {
	for {
		heapMutex.Lock()
		for scheduleHeap.Len() == 0 {
			heapCond.Wait()
			if ctx.Err() != nil {
				heapMutex.Unlock()
				return
			}
		}

		// Peek at the earliest schedule
		earliest := (*scheduleHeap)[0]
		waitDuration := earliest.NextRunTime.Sub(time.Now().UTC())

		if waitDuration <= 0 {
			// Pop and dispatch all schedules that are due
			dueSchedules := []models.Scheduler{}
			for scheduleHeap.Len() > 0 {
				earliest = (*scheduleHeap)[0]
				if earliest.NextRunTime.After(time.Now().UTC()) {
					break
				}
				schedule := heap.Pop(scheduleHeap).(models.Scheduler)
				dueSchedules = append(dueSchedules, schedule)
			}
			heapMutex.Unlock()

			// Dispatch due schedules
			for _, schedule := range dueSchedules {
				select {
				case processChan <- schedule:
				default:
					log.Printf("Process channel is full. Dropping schedule ID %s", schedule.ID)
				}
			}
		} else {
			// Wait until the next schedule is due or a new schedule is added
			timer := time.NewTimer(waitDuration)
			heapMutex.Unlock()

			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				// Timer is expired now, loop to dispatch due schedules
			}
		}
	}
}

// processWorker listens to the processChan and processes schedules
func processWorker(ctx context.Context, processChan <-chan models.Scheduler, workerID int) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d: Context canceled. Exiting processWorker.", workerID)
			return
		case schedule, ok := <-processChan:
			if !ok {
				log.Printf("Worker %d: Process channel closed. Exiting processWorker.", workerID)
				return
			}
			go processSchedule(schedule, workerID)
		}
	}
}

// processSchedule updates status and runs the scheduled webhook
func processSchedule(schedule models.Scheduler, workerID int) {
	log.Printf("Worker %d: Processing schedule: %s at %v", workerID, schedule.ID, time.Now())
	// Fetch the schedule from the database, only one time
	scheduleObjectID, err := primitive.ObjectIDFromHex(schedule.ID)
	if err != nil {
		log.Printf("Worker %d: Invalid schedule ID: %v", workerID, err)
		return
	}

	// Fetch the schedule from the database
	var fetchedSchedule models.Scheduler
	err = repository.SchedulerCollection.FindOne(context.Background(), bson.M{"_id": scheduleObjectID}).Decode(&fetchedSchedule)
	if err != nil {
		log.Printf("Worker %d: Error fetching schedule ID %s: %v", workerID, schedule.ID, err)
		return
	}

	if fetchedSchedule.Status != "processing" {
		log.Printf("Worker %d: Schedule ID %s is not in processing state", workerID, schedule.ID)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		// Mark status in-progress
		err := repository.UpdateSchedulerStatus(ctx, fetchedSchedule, "in-progress")
		if err != nil {
			log.Printf("Worker %d: Error updating status for schedule ID %s: %v", workerID, schedule.ID, err)
		}
	}()

	// Execute the webhook with context
	if err := executeWebhook(ctx, fetchedSchedule); err != nil {
		log.Printf("Worker %d: Error executing webhook for schedule ID %s: %v", workerID, fetchedSchedule.ID, err)
	} else {
		markProcessed(ctx, fetchedSchedule)
	}
}
