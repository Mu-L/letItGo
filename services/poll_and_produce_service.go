package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/Sumit189letItGo/repository"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/aws_msk_iam_v2"
)

const (
	fetchWindow          = 5 * time.Second
	maxFetchPerWin       = 1000
	kafkaTopic           = "scheduled_tasks"
	kafkaProducerRetries = 3
	kafkaBatchSize       = 100
	kafkaBatchTimeout    = 10 * time.Millisecond
	cacheWindow          = 10 * time.Minute
)

var kafkaBroker = os.Getenv("KAFKA_BROKER")

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
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("ap-south-1"))
	if err != nil {
		log.Fatalf("unable to load AWS config, %v", err)
	}

	mechanism := aws_msk_iam_v2.NewMechanism(cfg)
	dialer := &kafka.Dialer{
		Timeout:       10 * time.Second,
		DualStack:     true,
		TLS:           &tls.Config{},
		SASLMechanism: mechanism,
	}

	return &kafka.Writer{
		Addr:         kafka.TCP(kafkaBroker),
		Topic:        kafkaTopic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
		BatchSize:    kafkaBatchSize,
		BatchTimeout: kafkaBatchTimeout,
		Transport: &kafka.Transport{
			Dial: dialer.DialFunc,
		},
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
