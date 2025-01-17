package controllers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/Sumit189letItGo/models"
	"github.com/Sumit189letItGo/repository"
	"github.com/Sumit189letItGo/services"
)

func ScheduleHandler(w http.ResponseWriter, r *http.Request) {
	var tempPayload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&tempPayload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	payloadBytes, err := json.Marshal(tempPayload["payload"])
	if err != nil {
		http.Error(w, "Failed to encode payload", http.StatusInternalServerError)
		return
	}

	scheduler := models.NewScheduler()
	scheduler.WebhookURL = tempPayload["webhook_url"].(string)
	scheduler.Payload = string(payloadBytes)

	if tempPayload["time_as_text"] != nil {
		timeStringOrCronExp, isCron, err := repository.TextToTimeOrCronExpression(tempPayload["time_as_text"].(string))
		log.Println("timeStringOrCronExp", timeStringOrCronExp)
		if err != nil || timeStringOrCronExp == "" {
			http.Error(w, "Failed to convert text to time string or cron expression, Try Again with a better input", http.StatusBadRequest)
			return
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
			return
		}
		scheduler.ScheduleTime = &scheduleTime
	}

	if cronExpr, ok := tempPayload["cron_expression"].(string); ok {
		scheduler.CronExpression = cronExpr
	}

	if scheduler.ScheduleTime == nil && scheduler.CronExpression == "" {
		http.Error(w, "Either ScheduleTime or CronExpression must be provided", http.StatusBadRequest)
		return
	}

	if scheduler.ScheduleTime != nil {
		if scheduler.ScheduleTime.Location() != time.UTC {
			http.Error(w, "ScheduleTime must be in UTC", http.StatusBadRequest)
			return
		}
		t := scheduler.ScheduleTime.UTC()
		scheduler.ScheduleTime = &t
	}

	if cronExpr := scheduler.CronExpression; cronExpr != "" {
		if err := repository.ValidateCron(cronExpr); err != nil {
			http.Error(w, "Invalid cron expression: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	scheduled, err := services.Schedule(*scheduler)
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

func FetchPendingHandler(w http.ResponseWriter, r *http.Request) {
	limit := int64(10)
	tasks, err := services.FetchPendingSchedules(limit)
	if err != nil {
		http.Error(w, "Error fetching pending tasks: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}
