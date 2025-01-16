package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"sumit189/letItGo/models"
	"sumit189/letItGo/repository"
)

func Schedule(scheduler models.Scheduler) error {
	// Validation checks
	if scheduler.ScheduleTime == nil && scheduler.CronExpression == "" {
		return errors.New("either schedule_time or cron_expression must be provided")
	}
	if scheduler.ScheduleTime != nil && scheduler.CronExpression != "" {
		return errors.New("schedule_time and cron_expression cannot both be set")
	}

	scheduler.Status = "pending"
	scheduler.CreatedAt = time.Now()
	scheduler.UpdatedAt = time.Now()
	return repository.Schedule(scheduler)
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
	err := repository.RedisClient.SAdd(ctx, "processed_schedules", id).Err()
	if err != nil {
		log.Printf("Redis error: %v", err)
		return
	}

	_, err = repository.RedisClient.Expire(ctx, "processed_schedules", cacheWindow).Result()
	if err != nil {
		log.Printf("Redis error setting expiration: %v", err)
	}

	// mark completed
	err = repository.UpdateSchedulerStatus(schedule, "completed")

	if err != nil {
		log.Printf("Error updating status: %v", err)
	}
}

func executeWebhook(url string, payload interface{}, schedule models.Scheduler) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling payload: %v", err)
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("Error creating HTTP request: %v", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Error executing HTTP request: %v", err)
		repository.UpdateRetries(schedule.ID)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("Unexpected response status: %s", resp.Status)
		repository.UpdateRetries(schedule.ID)
		return errors.New("unexpected response status: " + resp.Status)
	}

	log.Printf("Webhook executed successfully: %s", resp.Status)
	return nil
}
