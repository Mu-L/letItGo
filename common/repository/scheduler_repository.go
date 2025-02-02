package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/Sumit189/letItGo/common/database"
	"github.com/Sumit189/letItGo/common/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var SchedulerCollection *mongo.Collection

func InitializeSchedulerRepository() {
	SchedulerCollection = database.GetCollection("schedulers")
}

func Schedule(ctx context.Context, scheduler models.Scheduler) (models.Scheduler, error) {
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

	insertedDoc, err := SchedulerCollection.InsertOne(ctx, newScheduler)
	if err != nil {
		return models.Scheduler{}, err
	}

	oid, ok := insertedDoc.InsertedID.(primitive.ObjectID)
	if ok {
		newScheduler.ID = oid.Hex()
	} else {
		return models.Scheduler{}, errors.New("insertedDoc.InsertedID is not of type ObjectID")
	}

	return *newScheduler, nil
}

func FetchPending(ctx context.Context, limit int64) ([]models.Scheduler, error) {
	currentTime := time.Now()
	timeWindowEnd := currentTime.Add(1 * time.Minute)

	filter := bson.M{
		"$or": []bson.M{
			{
				"status": "pending",
				"next_run_time": bson.M{
					"$lte": timeWindowEnd,
				},
			},
			{
				"status": "processing",
				"next_run_time": bson.M{
					"$gte": time.Now().Add(-5 * time.Minute),
				},
			},
		},
	}

	findOptions := options.Find()
	findOptions.SetLimit(limit)
	findOptions.SetSort(bson.M{"next_run_time": 1})

	cursor, err := SchedulerCollection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tasks []models.Scheduler
	for cursor.Next(ctx) {
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
		_, err := SchedulerCollection.BulkWrite(ctx, updateModels)
		if err != nil {
			log.Printf("Bulk update error to picked: %v", err)
		}
	}

	return tasks, nil
}

func UpdateSchedulerStatus(ctx context.Context, schedule models.Scheduler, status string) error {
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

	// Perform the update operation with context
	filter := bson.M{"_id": schedulerID}
	result, err := SchedulerCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update scheduler with ID %v: %w", schedule.ID, err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("no scheduler found with ID %v", schedule.ID)
	}

	// Handle cron rescheduling for "in-progress" status
	if status == "in-progress" && schedule.CronExpression != "" {
		return rescheduleCronJob(ctx, schedulerID)
	}

	return nil
}

func rescheduleCronJob(ctx context.Context, schedulerID primitive.ObjectID) error {
	// Find the updated schedule by ID using the provided context
	var updatedSchedule models.Scheduler
	err := SchedulerCollection.FindOne(ctx, bson.M{"_id": schedulerID}).Decode(&updatedSchedule)
	if err != nil {
		return fmt.Errorf("failed to find updated schedule with ID %v: %w", schedulerID.Hex(), err)
	}

	// Update the schedule for the next run
	updatedSchedule.Status = "pending"
	updatedSchedule.UpdatedAt = time.Now()
	updatedSchedule.ID = "" // Reset ID to avoid conflicts

	// Schedule the updated task with context support
	if _, err := Schedule(ctx, updatedSchedule); err != nil {
		return fmt.Errorf("failed to schedule cron for updated schedule with ID %v: %w", schedulerID.Hex(), err)
	}

	return nil
}

func UpdateRetries(ctx context.Context, schedule models.Scheduler) error {
	scheduleID, err := primitive.ObjectIDFromHex(schedule.ID)

	// Check if RetryLimit has been reached, marking failed
	if schedule.Retries >= schedule.RetryLimit {
		err = SendToArchive(ctx, schedule, "failed")
		if err != nil {
			log.Printf("Error sending to archive: %v", err)
		}
		return errors.New("retry limit reached")
	}

	nextRetryTime := time.Now().Add(time.Duration(schedule.RetryAfterInSeconds) * time.Second)
	_, err = SchedulerCollection.UpdateOne(
		ctx,
		bson.M{"_id": scheduleID},
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

func ExpireSchedules(ctx context.Context) error {
	findOptions := bson.M{
		"$or": []bson.M{
			{
				"status": bson.M{
					"$nin": []string{"completed", "failed"},
				},
				"next_run_time": bson.M{
					"$gte": time.Now().Add(-10 * time.Minute),
					"$lt":  time.Now().Add(-5 * time.Minute),
				},
			},
		},
	}

	cursor, err := SchedulerCollection.Find(ctx, findOptions)
	if err != nil {
		return err
	}

	var deadSchedules []models.Scheduler
	for cursor.Next(ctx) {
		var schedule models.Scheduler
		if err := cursor.Decode(&schedule); err != nil {
			return err
		}
		deadSchedules = append(deadSchedules, schedule)
	}
	if err := cursor.Err(); err != nil {
		return err
	}

	// Mark all dead schedules as failed
	updateModels := []mongo.WriteModel{}

	for _, schedule := range deadSchedules {
		objectID, err := primitive.ObjectIDFromHex(schedule.ID)
		if err != nil {
			return fmt.Errorf("invalid task ID: %v", err)
		}
		updateFilter := bson.M{"_id": objectID}
		update := bson.M{"$set": bson.M{"status": "failed"}}

		updateModel := mongo.NewUpdateOneModel().
			SetFilter(updateFilter).
			SetUpdate(update)
		updateModels = append(updateModels, updateModel)
	}

	if len(updateModels) > 0 {
		_, err := SchedulerCollection.BulkWrite(ctx, updateModels)
		if err != nil {
			log.Printf("Bulk update error to picked: %v", err)
		}

		for _, schedule := range deadSchedules {
			err = SendToArchive(ctx, schedule, "failed")
			if err != nil {
				log.Printf("Error sending to archive: %v", err)
			}
		}
	}

	return nil
}
