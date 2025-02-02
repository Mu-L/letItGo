package models

import "time"

type VerifiedWebhooks struct {
	ID         string    `json:"id,omitempty" bson:"_id,omitempty"`
	WebhookURL string    `json:"webhook_url" bson:"webhook_url"` // The URL to trigger
	MethodType string    `json:"method_type" bson:"method_type"` // HTTP method type
	Verified   bool      `json:"verified" bson:"verified"`       // Verified status
	CreatedAt  time.Time `json:"created_at" bson:"created_at"`   // Task creation timestamp
	UpdatedAt  time.Time `json:"updated_at" bson:"updated_at"`   // Last updated timestamp
}

func NewVerifiedWebhooks() *VerifiedWebhooks {
	return &VerifiedWebhooks{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
