package models

import (
	"context"

	"github.com/Sumit189/letItGo/common/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func CreateIndexes(ctx context.Context) {
	VerifiedWebhooks := database.GetCollection("verifiedwebhooks")
	VerifiedWebhooks.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"webhook_url": 1,
			"method_type": 1,
		},
	})
}
