package repository

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/Sumit189letItGo/database"
	"github.com/Sumit189letItGo/models"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var SchedulerCollection *mongo.Collection

func InitializeSchedulerRepository() {
	SchedulerCollection = database.GetCollection("schedulers")
}

func Schedule(scheduler models.Scheduler) error {
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
			return errors.New("invalid cron expression")
		}
		newScheduler.NextRunTime = &runTimeBasedOnCron
	} else {
		newScheduler.NextRunTime = scheduler.ScheduleTime
	}
	_, err := SchedulerCollection.InsertOne(context.Background(), newScheduler)
	return err
}

func FetchPending(limit int64) ([]models.Scheduler, error) {
	currentTime := time.Now()
	timeWindowEnd := currentTime.Add(1 * time.Minute)

	// Step 1: Retrieve IDs of schedules already in the queue from Redis
	queuedIDs, err := RedisClient.SMembers(context.Background(), "in_queue").Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}

	var excludedIDs []primitive.ObjectID
	for _, id := range queuedIDs {
		objectID, err := primitive.ObjectIDFromHex(id)
		if err == nil {
			excludedIDs = append(excludedIDs, objectID)
		}
	}

	// Step 2: Filter pending schedules based on time window and limit
	filter := bson.M{
		"status": "pending",
		"next_run_time": bson.M{
			"$lte": timeWindowEnd,
		},
	}

	if len(excludedIDs) > 0 {
		filter["_id"] = bson.M{
			"$nin": excludedIDs,
		}
	}

	findOptions := options.Find().SetLimit(limit)

	// Step 3: Fetch pending schedules
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

	// Step 4: Add fetched task IDs to Redis to mark them as in queue
	if len(tasks) > 0 {
		var idsToAdd []interface{}
		for _, task := range tasks {
			idsToAdd = append(idsToAdd, task.ID)
		}
		pipe := RedisClient.TxPipeline()
		pipe.SAdd(context.Background(), "in_queue", idsToAdd...)
		pipe.Expire(context.Background(), "in_queue", 2*time.Minute)
		_, err = pipe.Exec(context.Background())
		if err != nil {
			return nil, err
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
	if err := Schedule(updatedSchedule); err != nil {
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

	nextRetryTime := time.Now().Add(time.Duration(scheduler.RetryTimeoutInSeconds) * time.Second)
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
	return err
}
