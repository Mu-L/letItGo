package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/Sumit189letItGo/database"
	"github.com/Sumit189letItGo/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var SchedulerCollection *mongo.Collection

func InitializeSchedulerRepository() {
	SchedulerCollection = database.GetCollection("schedulers")
}

func Schedule(scheduler models.Scheduler) (models.Scheduler, error) {
	newScheduler := models.NewScheduler()

	// Use reflection to copy non-zero values from the provided scheduler
	schedulerValue := reflect.ValueOf(scheduler)
	newSchedulerValue := reflect.ValueOf(newScheduler).Elem()

	for i := 0; i < schedulerValue.NumField(); i++ {
		field := schedulerValue.Field(i)
		if !field.IsZero() {
			newSchedulerValue.Field(i).Set(field)
		}
	}
	newScheduler.UpdatedAt = time.Now()

	if scheduler.CronExpression != "" {
		runTimeBasedOnCron, err := CronToTime(scheduler.CronExpression)
		if err != nil {
			return models.Scheduler{}, errors.New("invalid cron expression")
		}
		newScheduler.NextRunTime = &runTimeBasedOnCron
	} else {
		newScheduler.NextRunTime = scheduler.ScheduleTime
	}

	insertedDoc, err := SchedulerCollection.InsertOne(context.Background(), newScheduler)
	if err != nil {
		return models.Scheduler{}, err
	}

	newScheduler.ID = insertedDoc.InsertedID.(primitive.ObjectID).Hex()
	return *newScheduler, nil
}

func FetchPending(limit int64) ([]models.Scheduler, error) {
	currentTime := time.Now()
	timeWindowEnd := currentTime.Add(1 * time.Minute)

	filter := bson.M{
		"status": "pending",
		"next_run_time": bson.M{
			"$lte": timeWindowEnd,
		},
	}

	findOptions := options.Find()
	findOptions.SetLimit(limit)
	findOptions.SetSort(bson.M{"next_run_time": 1})

	cursor, err := SchedulerCollection.Find(context.Background(), filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var tasks []models.Scheduler
	for cursor.Next(context.Background()) {
		var task models.Scheduler
		if err := cursor.Decode(&task); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}

	updateModels := []mongo.WriteModel{}

	for _, task := range tasks {
		objectID, err := primitive.ObjectIDFromHex(task.ID)
		if err != nil {
			return nil, fmt.Errorf("invalid task ID: %v", err)
		}
		updateFilter := bson.M{"_id": objectID}
		update := bson.M{"$set": bson.M{"status": "processing"}}

		updateModel := mongo.NewUpdateOneModel().
			SetFilter(updateFilter).
			SetUpdate(update)
		updateModels = append(updateModels, updateModel)
	}

	if len(updateModels) > 0 {
		_, err := SchedulerCollection.BulkWrite(context.Background(), updateModels)
		if err != nil {
			log.Printf("Bulk update error to picked: %v", err)
		}
	}

	return tasks, nil
}

func UpdateSchedulerStatus(schedule models.Scheduler, status string) error {
	// Ensure the ID is a valid ObjectID
	schedulerID, err := primitive.ObjectIDFromHex(schedule.ID)
	if err != nil {
		return fmt.Errorf("invalid task ID: %v", err)
	}

	// Prepare the update fields
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}

	if status == "in-progress" {
		update["$inc"] = bson.M{"run_count": 1}
	}

	// Perform the update operation
	filter := bson.M{"_id": schedulerID}
	result, err := SchedulerCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return fmt.Errorf("failed to update scheduler with ID %v: %w", schedule.ID, err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("no scheduler found with ID %v", schedule.ID)
	}

	// Handle cron rescheduling for "in-progress" status
	if status == "in-progress" && schedule.CronExpression != "" {
		return rescheduleCronJob(schedulerID)
	}

	fmt.Printf("Updated status to: %s for scheduler ID: %s\n", status, schedule.ID)
	return nil
}

func rescheduleCronJob(schedulerID primitive.ObjectID) error {
	// Find the updated schedule by ID
	var updatedSchedule models.Scheduler
	err := SchedulerCollection.FindOne(context.Background(), bson.M{"_id": schedulerID}).Decode(&updatedSchedule)
	if err != nil {
		return fmt.Errorf("failed to find updated schedule with ID %v: %w", schedulerID.Hex(), err)
	}

	// Update the schedule for the next run
	updatedSchedule.Status = "pending"
	updatedSchedule.UpdatedAt = time.Now()
	updatedSchedule.ID = "" // Reset ID to avoid conflicts

	// Schedule the updated task
	if _, err := Schedule(updatedSchedule); err != nil {
		return fmt.Errorf("failed to schedule cron for updated schedule with ID %v: %w", schedulerID.Hex(), err)
	}

	return nil
}

func UpdateRetries(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid task ID")
	}

	var scheduler models.Scheduler
	schedulerErr := SchedulerCollection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&scheduler)
	if schedulerErr != nil {
		return errors.New("task not found")
	}

	// Check if RetryLimit has been reached, marking failed
	if scheduler.Retries >= scheduler.RetryLimit {
		_, err = SchedulerCollection.UpdateOne(
			context.Background(),
			bson.M{"_id": objectID},
			bson.M{
				"$set": bson.M{
					"status":     "failed",
					"updated_at": time.Now(),
				},
			},
		)
		return errors.New("retry limit reached")
	}

	nextRetryTime := time.Now().Add(time.Duration(scheduler.RetryAfterInSeconds) * time.Second)
	_, err = SchedulerCollection.UpdateOne(
		context.Background(),
		bson.M{"_id": objectID},
		bson.M{
			"$inc": bson.M{"retries": 1},
			"$set": bson.M{
				"status":        "pending",
				"next_run_time": nextRetryTime,
				"updated_at":    time.Now(),
			},
		},
	)
	if err != nil {
		return errors.New("error updating retries")
	}
	return nil
}
