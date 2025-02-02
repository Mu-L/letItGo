package services

import (
	"context"
	"errors"

	"github.com/Sumit189/letItGo/common/models"
	"github.com/Sumit189/letItGo/common/repository"
)

func FetchPendingSchedules(ctx context.Context, limit int64) ([]models.Scheduler, error) {
	if limit <= 0 {
		return nil, errors.New("limit must be greater than zero")
	}

	tasks, err := repository.FetchPending(ctx, limit)
	if err != nil {
		return nil, err
	}

	return tasks, nil
}
