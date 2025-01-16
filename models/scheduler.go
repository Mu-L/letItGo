package models

import "time"

// Scheduler represents a task to trigger a webhook at a scheduled time or based on a cron expression.
type Scheduler struct {
	ID                    string     `json:"id,omitempty" bson:"_id,omitempty"`
	WebhookURL            string     `json:"webhook_url" bson:"webhook_url"`                             // The URL to trigger
	Payload               string     `json:"payload" bson:"payload"`                                     // Encrypted payload to send
	ScheduleTime          *time.Time `json:"schedule_time" bson:"schedule_time"`                         // Specific time for one-time triggers (pointer to handle nil)
	CronExpression        string     `json:"cron_expression,omitempty" bson:"cron_expression,omitempty"` // Cron for recurring schedules (optional)
	NextRunTime           *time.Time `json:"next_run_time,omitempty" bson:"next_run_time,omitempty"`     // Next run time for cron schedules
	Status                string     `json:"status" bson:"status"`                                       // pending, in-progress, completed, failed
	Retries               int        `json:"retries" bson:"retries"`                                     // Number of retries
	RetryLimit            int        `json:"retry_limit" bson:"retry_limit"`                             // Retry limit
	RetryTimeoutInSeconds int        `json:"retry_timeout_in_seconds" bson:"retry_timeout_in_seconds"`   // Retry timeout in seconds
	RunCount              int        `json:"run_count" bson:"run_count"`                                 // Number of times the task has been run
	CreatedAt             time.Time  `json:"created_at" bson:"created_at"`                               // Task creation timestamp
	UpdatedAt             time.Time  `json:"updated_at" bson:"updated_at"`                               // Last updated timestamp
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		RetryLimit:            3,
		RetryTimeoutInSeconds: 30,
		Status:                "pending",
		RunCount:              0,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
}
