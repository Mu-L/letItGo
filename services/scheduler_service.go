package services

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/Sumit189letItGo/models"
	"github.com/Sumit189letItGo/repository"
	"github.com/Sumit189letItGo/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	allowedStatusCodes = map[int]bool{
		408: true,
		429: true,
		500: true,
		502: true,
		503: true,
		504: true,
	}
	sharedClient = &http.Client{Timeout: 10 * time.Second}
)

func Schedule(scheduler models.Scheduler) (models.Scheduler, error) {
	// Validation checks
	if scheduler.ScheduleTime == nil && scheduler.CronExpression == "" {
		return models.Scheduler{}, errors.New("either schedule_time or cron_expression must be provided")
	}
	if scheduler.ScheduleTime != nil && scheduler.CronExpression != "" {
		return models.Scheduler{}, errors.New("schedule_time and cron_expression cannot both be set")
	}

	// Encrypt the payload
	encryptedPayload, err := utils.Encrypt(scheduler.Payload)
	if err != nil {
		return models.Scheduler{}, err
	}
	scheduler.Payload = encryptedPayload

	scheduler.Status = "pending"
	scheduler.CreatedAt = time.Now()
	scheduler.UpdatedAt = time.Now()
	scheduled, err := repository.Schedule(scheduler)
	if err != nil {
		return models.Scheduler{}, err
	}
	return scheduled, nil
}

func FetchPendingSchedules(limit int64) ([]models.Scheduler, error) {
	if limit <= 0 {
		return nil, errors.New("limit must be greater than zero")
	}

	tasks, err := repository.FetchPending(limit)
	if err != nil {
		return nil, err
	}

	return tasks, nil
}

func isProcessed(id string) bool {
	exists, err := repository.RedisClient.SIsMember(ctx, "processed_schedules", id).Result()
	if err != nil {
		log.Printf("Redis error: %v", err)
		return false
	}
	return exists
}

func markProcessed(schedule models.Scheduler) {
	id := schedule.ID
	pipe := repository.RedisClient.TxPipeline()

	pipe.SAdd(ctx, "processed_schedules", id)
	pipe.Expire(ctx, "processed_schedules", cacheWindow)

	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("Redis error: %v", err)
		return
	}

	// mark completed
	err = repository.UpdateSchedulerStatus(schedule, "completed")
	if err != nil {
		log.Printf("Error updating status: %v", err)
	}
}

func executeWebhook(scheduleId string) error {
	var schedule models.Scheduler
	scheduleObjectID, err := primitive.ObjectIDFromHex(scheduleId)
	if err != nil {
		log.Printf("Invalid schedule ID: %v", err)
		return err
	}

	err = repository.SchedulerCollection.FindOne(ctx, bson.M{"_id": scheduleObjectID}).Decode(&schedule)
	if err != nil {
		log.Printf("Error decoding schedule: %v", err)
		return err
	}

	for {
		payloadBytes, err := utils.DecryptAndConvertToJSON(schedule.Payload)
		if err != nil {
			log.Printf("Error decrypting payload: %v", err)
			repository.UpdateRetries(schedule.ID)
			return err
		}

		req, err := http.NewRequest(schedule.MethodType, schedule.WebhookURL, bytes.NewReader(payloadBytes.([]byte)))
		if err != nil {
			log.Printf("Error creating request: %v", err)
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := sharedClient.Do(req)
		if err != nil {
			log.Printf("HTTP request error: %v", err)
			updateErr := repository.UpdateRetries(schedule.ID)
			if updateErr != nil {
				log.Printf("Error updating retries: %v", updateErr)
				return updateErr
			}
			return nil
		}

		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("Webhook executed successfully: %s", resp.Status)
			return nil
		}

		log.Printf("Unexpected response status: %s", resp.Status)
		if !allowedStatusCodes[resp.StatusCode] {
			return errors.New("unexpected response: " + resp.Status)
		}

		if schedule.WebhookRetryCount >= schedule.WebhookRetryLimit {
			log.Printf("Webhook retry limit reached for schedule ID %s", schedule.ID)
			repository.UpdateRetries(schedule.ID)
			return errors.New("webhook retry limit reached")
		}

		// Introduce delay before retrying
		time.Sleep(time.Duration(schedule.RetryAfterInSeconds) * time.Second)

		err = repository.SchedulerCollection.FindOneAndUpdate(
			ctx,
			bson.M{"_id": scheduleObjectID},
			bson.M{"$inc": bson.M{"webhook_retry_count": 1}},
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		).Decode(&schedule)
		if err != nil {
			log.Printf("Error incrementing and fetching updated schedule: %v", err)
			return err
		}

		log.Printf("Retry attempt %d for schedule ID %s", schedule.WebhookRetryCount, schedule.ID)
	}
}
