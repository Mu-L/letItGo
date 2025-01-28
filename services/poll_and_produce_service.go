package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/Sumit189letItGo/repository"
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
)

var kafkaBrokers = []string{os.Getenv("KAFKA_BROKER")}

type MSKAccessTokenProvider struct {
}

func (m *MSKAccessTokenProvider) Token() (*sarama.AccessToken, error) {
	token, _, err := signer.GenerateAuthToken(context.TODO(), "<region>")
	return &sarama.AccessToken{Token: token}, err
}

func setupProducer(brokers []string) (sarama.AsyncProducer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true

	config.Net.SASL.Enable = true
	config.Net.SASL.Mechanism = sarama.SASLTypeOAuth
	config.Net.SASL.TokenProvider = &MSKAccessTokenProvider{}

	config.Net.TLS.Enable = true
	config.Net.TLS.Config = &tls.Config{
		InsecureSkipVerify: false,
	}

	producer, err := sarama.NewAsyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}

	return producer, nil
}

func PollAndProduce(ctx context.Context) {
	log.Println("Starting Producer Service...")
	producer, err := setupProducer(kafkaBrokers)
	if err != nil {
		log.Fatalf("Failed to setup Kafka producer: %v", err)
	}
	defer func() {
		if err := producer.Close(); err != nil {
			log.Printf("Error closing Kafka producer: %v", err)
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
			err := publishDueSchedules(ctx, producer)
			if err != nil {
				log.Printf("Error publishing due schedules: %v", err)
			}
		}
	}
}

func publishDueSchedules(ctx context.Context, producer sarama.AsyncProducer) error {
	log.Println("Fetching pending schedules...")
	schedules, err := repository.FetchPending(ctx, int64(maxFetchPerWin))
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
	}

	return nil
}
