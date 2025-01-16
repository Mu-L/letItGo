package controllers

import (
	"encoding/json"
	"net/http"
	"sumit189/letItGo/models"
	"sumit189/letItGo/repository"
	"sumit189/letItGo/services"
	"time"
)

func ScheduleHandler(w http.ResponseWriter, r *http.Request) {
	var scheduler models.Scheduler
	err := json.NewDecoder(r.Body).Decode(&scheduler)
	if err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
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

	err = services.Schedule(scheduler)
	if err != nil {
		http.Error(w, "Error scheduling webhook: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Task scheduled"})
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
