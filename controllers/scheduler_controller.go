package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Sumit189letItGo/models"
	"github.com/Sumit189letItGo/repository"
	"github.com/Sumit189letItGo/services"
	"github.com/Sumit189letItGo/utils"
)

func ScheduleHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	scheduler, err := parseAndValidatePayload(w, r)
	if err != nil {
		return
	}

	scheduled, err := services.Schedule(ctx, *scheduler)
	if err != nil {
		http.Error(w, "Error scheduling webhook: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	timeStr := ""
	if scheduled.NextRunTime != nil {
		timeStr = scheduled.NextRunTime.Format(time.RFC3339)
	}
	json.NewEncoder(w).Encode(map[string]string{"message": "Task scheduled", "time": timeStr, "cron": scheduled.CronExpression, "id": scheduled.ID})
}

func parseAndValidatePayload(w http.ResponseWriter, r *http.Request) (*models.Scheduler, error) {
	var tempPayload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&tempPayload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return nil, err
	}

	scheduler := models.NewScheduler()
	if err := utils.ValidateAndAssignStringField(tempPayload, "webhook_url", &scheduler.WebhookURL, w); err != nil {
		return nil, err
	}
	if err := utils.ValidateAndAssignStringField(tempPayload, "method_type", &scheduler.MethodType, w); err != nil {
		return nil, err
	}

	payloadBytes, err := json.Marshal(tempPayload["payload"])
	if err != nil {
		http.Error(w, "Failed to encode payload", http.StatusInternalServerError)
		return nil, err
	}
	scheduler.Payload = string(payloadBytes)

	if timeAsText, ok := tempPayload["time_as_text"].(string); ok {
		timeStringOrCronExp, isCron, err := repository.TextToTimeOrCronExpression(timeAsText)
		if err != nil || timeStringOrCronExp == "" {
			http.Error(w, "Failed to convert text to time string or cron expression", http.StatusBadRequest)
			return nil, err
		}

		if isCron {
			tempPayload["cron_expression"] = timeStringOrCronExp
		} else {
			tempPayload["schedule_time"] = timeStringOrCronExp
		}
	}

	if scheduleTimeStr, ok := tempPayload["schedule_time"].(string); ok {
		scheduleTime, err := time.Parse(time.RFC3339, scheduleTimeStr)
		if err != nil {
			http.Error(w, "Invalid schedule_time format", http.StatusBadRequest)
			return nil, err
		}
		scheduler.ScheduleTime = &scheduleTime
	}

	if cronExpr, ok := tempPayload["cron_expression"].(string); ok {
		scheduler.CronExpression = cronExpr
	}

	if scheduler.ScheduleTime == nil && scheduler.CronExpression == "" {
		err := errors.New("either ScheduleTime or CronExpression must be provided")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil, err
	}

	if scheduler.ScheduleTime != nil && scheduler.ScheduleTime.Location() != time.UTC {
		err := errors.New("ScheduleTime must be in UTC")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil, err
	}

	if cronExpr := scheduler.CronExpression; cronExpr != "" {
		if err := repository.ValidateCron(cronExpr); err != nil {
			http.Error(w, "Invalid cron expression: "+err.Error(), http.StatusBadRequest)
			return nil, err
		}
	}

	return scheduler, nil
}
