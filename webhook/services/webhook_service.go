package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Sumit189/letItGo/common/models"
	"github.com/Sumit189/letItGo/common/repository"
	"github.com/Sumit189/letItGo/common/utils"
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

func Schedule(ctx context.Context, scheduler models.Scheduler) (models.Scheduler, error) {
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
	scheduled, err := repository.Schedule(ctx, scheduler)
	if err != nil {
		return models.Scheduler{}, err
	}
	return scheduled, nil
}
