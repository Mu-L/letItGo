package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/Sumit189/letItGo/common/repository"
	"github.com/aws/aws-msk-iam-sasl-signer-go/signer"
)

const (
	fetchWindow          = 5 * time.Second
	maxFetchPerWin       = 1000
	kafkaTopic           = "scheduled_tasks"
	kafkaProducerRetries = 3
	kafkaBatchSize       = 100
	kafkaBatchTimeout    = 10 * time.Millisecond
	cacheWindow          = 10 * time.Minute
	expireScheduleWindow = 10 * time.Minute
)

type MSKAccessTokenProvider struct {
}

func (m *MSKAccessTokenProvider) Token() (*sarama.AccessToken, error) {
	token, _, err := signer.GenerateAuthToken(context.TODO(), "ap-south-1")
	return &sarama.AccessToken{Token: token}, err
}

func setupProducer(brokers []string) (sarama.AsyncProducer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true
	if os.Getenv("ENVIRONMENT") == "development" {
		// Development setup with local Kafka
	} else {
		// Production setup with AWS MSK
		config.Net.SASL.Enable = true
		config.Net.SASL.Mechanism = sarama.SASLTypeOAuth
		config.Net.SASL.TokenProvider = &MSKAccessTokenProvider{}
		config.Net.TLS.Enable = true
		config.Net.TLS.Config = &tls.Config{
			InsecureSkipVerify: false,
		}
	}
	producer, err := sarama.NewAsyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}

	return producer, nil
}

func PollAndProduce(ctx context.Context) {
	var kafkaBrokers = []string{os.Getenv("KAFKA_BROKER")}
	log.Println("Starting Producer Service...")

	// Add detailed logging
	if kafkaBrokers[0] == "" {
		log.Fatalf("KAFKA_BROKER environment variable not set")
	}
	log.Printf("Attempting to connect to Kafka brokers: %v", kafkaBrokers)

	producer, err := setupProducer(kafkaBrokers)
	if err != nil {
		log.Fatalf("Failed to setup Kafka producer: %v", err)
	}
	defer func() {
		if err := producer.Close(); err != nil {
			log.Printf("Error closing Kafka producer: %v", err)
		}
	}()

	// Add error channel handling
	go func() {
		for err := range producer.Errors() {
			log.Printf("Producer error: %v", err)
		}
	}()

	ticker := time.NewTicker(fetchWindow)
	expireTicker := time.NewTicker(expireScheduleWindow)

	defer ticker.Stop()
	defer expireTicker.Stop()

	log.Println("Successfully connected to Kafka!")
	log.Println("Started Producer Service...")
	log.Println("__________________________")
	log.Println("Polling and producing schedules...")
	for {
		select {
		case <-ctx.Done():
			log.Println("PollAndProduce received context cancellation. Exiting...")
			return
		case <-ticker.C:
			err := publishDueSchedules(ctx, producer)
			if err != nil {
				log.Printf("Error publishing due schedules: %v", err)
			}
		case <-expireTicker.C:
			err := repository.ExpireSchedules(ctx)
			if err != nil {
				log.Printf("Error expiring schedules: %v", err)
			}
		}
	}
}

func publishDueSchedules(ctx context.Context, producer sarama.AsyncProducer) error {
	schedules, err := FetchPendingSchedules(ctx, int64(maxFetchPerWin))
	if err != nil {
		return err
	}

	if len(schedules) == 0 {
		return nil
	}

	for _, schedule := range schedules {
		bytes, err := json.Marshal(schedule)
		if err != nil {
			log.Printf("Error marshaling schedule ID %s: %v", schedule.ID, err)
			continue
		}

		msg := &sarama.ProducerMessage{
			Topic: kafkaTopic,
			Key:   sarama.StringEncoder(schedule.ID),
			Value: sarama.ByteEncoder(bytes),
		}
		producer.Input() <- msg
		log.Printf("Published schedule ID %s to Kafka", schedule.ID)
	}

	return nil
}
