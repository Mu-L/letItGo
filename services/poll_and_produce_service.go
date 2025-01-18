package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/Sumit189letItGo/repository"
	"github.com/segmentio/kafka-go"
)

const (
	fetchWindow          = 5 * time.Second
	maxFetchPerWin       = 1000
	kafkaBroker          = "localhost:9092"
	kafkaTopic           = "scheduled_tasks"
	kafkaProducerRetries = 3
	kafkaBatchSize       = 100
	kafkaBatchTimeout    = 10 * time.Millisecond
	cacheWindow          = 10 * time.Minute
)

func PollAndProduce(ctx context.Context) {
	log.Println("Starting Producer Service...")

	writer := initKafkaWriter()
	defer func() {
		if err := writer.Close(); err != nil {
			log.Printf("Error closing Kafka writer: %v", err)
		}
	}()

	ticker := time.NewTicker(fetchWindow)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("PollAndProduce received context cancellation. Exiting...")
			return
		case <-ticker.C:
			err := publishDueSchedules(ctx, writer)
			if err != nil {
				log.Printf("Error publishing due schedules: %v", err)
			}
		}
	}
}

// Initialize Kafka writer
func initKafkaWriter() *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(kafkaBroker),
		Topic:        kafkaTopic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
		BatchSize:    kafkaBatchSize,
		BatchTimeout: kafkaBatchTimeout,
	}
}

// Publish due schedules to Kafka
func publishDueSchedules(ctx context.Context, writer *kafka.Writer) error {
	schedules, err := repository.FetchPending(ctx, int64(maxFetchPerWin))
	if err != nil {
		return err
	}

	if len(schedules) == 0 {
		return nil
	}

	var messages []kafka.Message
	for _, schedule := range schedules {
		bytes, err := json.Marshal(schedule)
		if err != nil {
			log.Printf("Error marshaling schedule ID %s: %v", schedule.ID, err)
			continue
		}

		msg := kafka.Message{
			Key:   []byte(schedule.ID),
			Value: bytes,
			Time:  time.Now(),
		}
		messages = append(messages, msg)
	}

	// Retry logic with exponential backoff
	for attempt := 0; attempt < kafkaProducerRetries; attempt++ {
		err = writer.WriteMessages(ctx, messages...)
		if err == nil {
			log.Printf("Successfully published %d schedules to Kafka", len(messages))
			return nil
		}
		log.Printf("Attempt %d: Failed to publish schedules: %v", attempt+1, err)
		time.Sleep(time.Duration(attempt+1) * time.Second) // Exponential backoff
	}

	return err
}
