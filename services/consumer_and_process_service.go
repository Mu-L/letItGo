package services

import (
	"container/heap"
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Sumit189letItGo/models"
	"github.com/Sumit189letItGo/repository"
	"github.com/segmentio/kafka-go"
)

const (
	workerCount             = 20
	consumerGroupID         = "schedule_processor_group"
	kafkaVersion            = "2.1.0"
	kafkaMinBytes           = 10e3 // 10KB
	kafkaMaxBytes           = 10e6 // 10MB
	kafkaCommitInterval     = time.Second
	processChanBuffer       = 10000 // Adjust buffer size as needed
	shutdownTimeout         = 30 * time.Second
	scheduleDispatchTimeout = 1 * time.Second
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

func ConsumeAndProcess(ctx context.Context) {
	log.Println("Starting Consumer Service...")

	// Initialize Kafka reader
	reader := initKafkaReader()
	defer func() {
		if err := reader.Close(); err != nil {
			log.Printf("Error closing Kafka reader: %v", err)
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
			consume(ctx, reader, workerID, scheduleHeap, &heapMutex, heapCond)
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

// Initialize Kafka reader
func initKafkaReader() *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{kafkaBroker},
		GroupID:        consumerGroupID,
		Topic:          kafkaTopic,
		MinBytes:       kafkaMinBytes,    // 10KB
		MaxBytes:       kafkaMaxBytes,    // 10MB
		StartOffset:    kafka.LastOffset, // Start from the latest offset
		MaxAttempts:    3,
		CommitInterval: kafkaCommitInterval, // Commit offsets every second
	})
}

// consume reads messages from Kafka and adds them to the schedule heap
func consume(ctx context.Context, reader *kafka.Reader, workerID int, scheduleHeap *ScheduleHeap, heapMutex *sync.Mutex, heapCond *sync.Cond) {
	for {
		m, err := reader.FetchMessage(ctx)
		if err != nil {
			if err == context.Canceled {
				log.Printf("Worker %d: Context canceled. Exiting consume loop.", workerID)
				return
			}
			log.Printf("Worker %d: Error fetching message: %v", workerID, err)
			time.Sleep(time.Second) // Backoff on error
			continue
		}

		var schedule models.Scheduler
		err = json.Unmarshal(m.Value, &schedule)
		if err != nil {
			log.Printf("Worker %d: Error unmarshaling schedule ID %s: %v", workerID, string(m.Key), err)
			// Even if unmarshaling fails, mark message as processed to avoid retrying
			if commitErr := reader.CommitMessages(ctx, m); commitErr != nil {
				log.Printf("Worker %d: Failed to commit message: %v", workerID, commitErr)
			}
			continue
		}

		heapMutex.Lock()
		heap.Push(scheduleHeap, schedule)
		heapCond.Signal() // Notify the scheduler that a new schedule has been added
		heapMutex.Unlock()

		// Commit the message
		if err := reader.CommitMessages(ctx, m); err != nil {
			log.Printf("Worker %d: Failed to commit message ID %s: %v", workerID, schedule.ID, err)
		}
	}
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
		now := time.Now()
		waitDuration := earliest.NextRunTime.Sub(now)

		if waitDuration <= 0 {
			// Pop and dispatch all schedules that are due
			dueSchedules := []models.Scheduler{}
			for scheduleHeap.Len() > 0 {
				earliest = (*scheduleHeap)[0]
				if earliest.NextRunTime.After(time.Now()) {
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
			processSchedule(schedule, workerID)
		}
	}
}

// processSchedule updates status and runs the scheduled webhook
func processSchedule(schedule models.Scheduler, workerID int) {
	log.Printf("Worker %d: Processing schedule: %s at %v", workerID, schedule.ID, time.Now())

	// Create a context with timeout for processing the schedule
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Mark status in-progress
	err := repository.UpdateSchedulerStatus(ctx, schedule, "in-progress")
	if err != nil {
		log.Printf("Worker %d: Error updating status for schedule ID %s: %v", workerID, schedule.ID, err)
		return
	}

	// Execute the webhook with context
	if err := executeWebhook(ctx, schedule.ID); err != nil {
		log.Printf("Worker %d: Error executing webhook for schedule ID %s: %v", workerID, schedule.ID, err)
	} else {
		markProcessed(ctx, schedule)
	}
}
