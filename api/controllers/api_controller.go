package controllers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Sumit189/letItGo/common/models"
	"github.com/Sumit189/letItGo/common/repository"
	"github.com/Sumit189/letItGo/common/utils"
	"github.com/Sumit189/letItGo/consumer/services"
)

func ScheduleHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	scheduler, err := parseAndValidatePayload(ctx, w, r)
	if err != nil {
		return
	}

	if scheduler == nil {
		return
	}

	scheduled, err := services.Schedule(ctx, *scheduler)
	if err != nil {
		http.Error(w, "Error scheduling webhook: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if scheduled.ID == "" {
		http.Error(w, "Failed to schedule webhook", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	timeStr := ""
	if scheduled.NextRunTime != nil {
		timeStr = scheduled.NextRunTime.Format(time.RFC3339)
	}
	json.NewEncoder(w).Encode(map[string]string{"message": "Task scheduled", "time": timeStr, "cron": scheduled.CronExpression, "id": scheduled.ID})
}

func parseAndValidatePayload(ctx context.Context, w http.ResponseWriter, r *http.Request) (*models.Scheduler, error) {
	var tempPayload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&tempPayload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return nil, err
	}

	scheduler := models.NewScheduler()
	if err := utils.ValidateAndAssignStringField(ctx, tempPayload, "webhook_url", &scheduler.WebhookURL, w); err != nil {
		return nil, err
	}
	if err := utils.ValidateAndAssignStringField(ctx, tempPayload, "method_type", &scheduler.MethodType, w); err != nil {
		return nil, err
	}

	// check if webhook_url and method_type are valid
	IsVerifiedWebhook := repository.IsVerifiedWebhook(ctx, scheduler.WebhookURL, scheduler.MethodType)
	if !IsVerifiedWebhook {
		http.Error(w, "Webhook is not verified", http.StatusBadRequest)
		return nil, errors.New("Webhook is not verified")
	}

	payloadBytes, err := json.Marshal(tempPayload["payload"])
	if err != nil {
		http.Error(w, "Failed to encode payload", http.StatusInternalServerError)
		return nil, err
	}
	scheduler.Payload = string(payloadBytes)

	if timeAsText, ok := tempPayload["time_as_text"].(string); ok {
		timeStringOrCronExp, isCron, err := repository.TextToTimeOrCronExpression(ctx, timeAsText)
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

		// error out if the schedule time is in the past
		if scheduleTime.Before(time.Now().UTC()) {
			err := errors.New("schedule_time must be in the future")
			http.Error(w, err.Error(), http.StatusBadRequest)
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

func VerifyWebhookHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	fmt.Println("Payload: ", payload)

	if len(payload) == 0 {
		http.Error(w, "Empty payload", http.StatusBadRequest)
		return
	}

	webhookURL := payload["webhook_url"].(string)
	if webhookURL == "" {
		http.Error(w, "Missing webhook URL", http.StatusBadRequest)
		return
	}

	methodType := payload["method_type"].(string)
	if methodType == "" {
		http.Error(w, "Missing method type: [\"GET\", \"POST\"]", http.StatusBadRequest)
		return
	}

	// Check if the webhook URL is already verified
	isVerified := repository.IsVerifiedWebhook(ctx, webhookURL, methodType)
	if isVerified {
		http.Error(w, "Webhook already verified", http.StatusBadRequest)
		return
	}

	secretKey := os.Getenv("WEBHOOK_SECRET_KEY")

	// Generate the signature based on the webhook URL
	secureSignature := GenerateSignature(webhookURL, secretKey)

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// check headers for the signature
	// Compare the signatures
	signature := resp.Header.Get("X-Webhook-Signature")
	if secureSignature != signature {
		http.Error(w, "Webhook verification failed", http.StatusBadRequest)
		return
	}

	repository.AddVerifiedWebhook(ctx, webhookURL, methodType)

	// Respond with a success message after one-time verification
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Webhook successfully verified"))
}

func GenerateSignature(url, secretKey string) string {
	// Create a secure signature using HMAC with SHA256
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(url))
	return hex.EncodeToString(mac.Sum(nil))
}
