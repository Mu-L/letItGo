package repository

import (
	"context"
	"os"

	"github.com/Sumit189/letItGo/common/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var VerifiedWebhooks *mongo.Collection

func InitializeVerifiedWebhooksRepository() {
	VerifiedWebhooks = database.GetCollection("verified_webhooks")
}

func IsVerifiedWebhook(ctx context.Context, webhookURL string, methodType string) bool {
	if os.Getenv("ENVIRONMENT") == "development" {
		return true
	}
	var result bson.M
	err := VerifiedWebhooks.FindOne(ctx, bson.M{"webhook_url": webhookURL, "method_type": methodType, "verified": true}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false
		}
		return false
	}

	return true
}

func AddVerifiedWebhook(ctx context.Context, webhookURL string, methodType string) error {
	_, err := VerifiedWebhooks.InsertOne(ctx, bson.M{"webhook_url": webhookURL, "method_type": methodType, "verified": true})
	if err != nil {
		return err
	}
	return nil
}
