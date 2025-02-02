package repository

import (
	"errors"
	"time"

	"github.com/robfig/cron/v3"
)

func ValidateCron(CronExpression string) error {
	_, err := cron.ParseStandard(CronExpression)
	if err != nil {
		return errors.New("invalid cron expression")
	}
	return nil
}

func CronToTime(CronExpression string) (time.Time, error) {
	cronData, err := cron.ParseStandard(CronExpression)
	if err != nil {
		return time.Time{}, errors.New("invalid cron expression")
	}
	var nextRunTime = cronData.Next(time.Now().Add(time.Second))
	return nextRunTime, nil
}
