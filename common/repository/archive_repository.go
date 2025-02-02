package repository

import (
	"context"

	"github.com/Sumit189/letItGo/common/database"
	"github.com/Sumit189/letItGo/common/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var ArchiveCollection *mongo.Collection

func InitializeArchiveRepository() {
	ArchiveCollection = database.GetCollection("archives")
}

func SendToArchive(ctx context.Context, toBeArchived models.Scheduler, status string) error {
	scheduleId, err := primitive.ObjectIDFromHex(toBeArchived.ID)
	if err != nil {
		return err
	}

	toBeArchived.Status = status
	insertedDoc, err := ArchiveCollection.InsertOne(ctx, toBeArchived)
	if err != nil {
		return err
	}

	if insertedDoc != nil {
		_, err = SchedulerCollection.DeleteOne(ctx, bson.M{"_id": scheduleId})
		if err != nil {
			return err
		}
	}
	return nil
}
