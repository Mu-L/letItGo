package services

import (
	"container/heap"
	"context"
	"log"
	"sort"
	"time"

	"github.com/Sumit189letItGo/models"
	"github.com/Sumit189letItGo/repository"

	"github.com/redis/go-redis/v9"
)

const (
	workerCount    = 5
	queueSize      = 100
	cacheWindow    = 1 * time.Minute
	fetchWindow    = 5 * time.Second
	maxFetchPerWin = 10
)

var (
	ctx = context.Background()
)

type ScheduleHeap []models.Scheduler

func (h ScheduleHeap) Len() int           { return len(h) }
func (h ScheduleHeap) Less(i, j int) bool { return h[i].NextRunTime.Before(*h[j].NextRunTime) }
func (h ScheduleHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *ScheduleHeap) Push(x interface{}) {
	*h = append(*h, x.(models.Scheduler))
}

func (h *ScheduleHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

func PollSchedules() {
	log.Println("Polling schedules")
	scheduleQueue := make(chan models.Scheduler, queueSize)
	log.Println("Schedule queue created")

	// Start worker goroutines
	for i := 0; i < workerCount; i++ {
		log.Println("Starting worker: ", i)
		go worker(scheduleQueue)
	}

	resetTicker := time.NewTicker(fetchWindow)
	defer resetTicker.Stop()

	for {
		select {
		case <-resetTicker.C:
			// Counter reset
			err := repository.RedisClient.Set(ctx, "fetch_count", 0, 0).Err()
			if err != nil {
				log.Printf("Redis error resetting fetch count: %v", err)
			}
		default:
			count, err := repository.RedisClient.Get(ctx, "fetch_count").Int()
			if err != nil && err != redis.Nil {
				log.Printf("Redis error getting fetch count: %v", err)
				time.Sleep(fetchWindow)
				continue
			}

			if count >= maxFetchPerWin {
				time.Sleep(fetchWindow)
				continue
			}

			err = repository.RedisClient.Incr(ctx, "fetch_count").Err()
			if err != nil {
				log.Printf("Redis error incrementing fetch count: %v", err)
				time.Sleep(fetchWindow)
				continue
			}

			// Fetch pending schedules from db
			schedules, err := repository.FetchPending(10)
			log.Printf("Fetched schedules: %v", len(schedules))
			if err != nil {
				log.Printf("Error fetching pending schedules: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			sort.Slice(schedules, func(i, j int) bool {
				return schedules[i].NextRunTime.Before(*schedules[j].NextRunTime)
			})

			for _, schedule := range schedules {
				if isProcessed(schedule.ID) {
					continue
				}
				scheduleQueue <- schedule
			}
			// Wait before the next polling cycle
			time.Sleep(fetchWindow)
		}
	}
}

func worker(queue chan models.Scheduler) {
	log.Println("Worker started")
	scheduleHeap := &ScheduleHeap{}
	heap.Init(scheduleHeap)

	for {
		select {
		case schedule := <-queue:
			heap.Push(scheduleHeap, schedule)
		default:
			if scheduleHeap.Len() > 0 {
				nextSchedule := heap.Pop(scheduleHeap).(models.Scheduler)
				nextRunTime := nextSchedule.NextRunTime
				time.AfterFunc(time.Until(*nextRunTime), func() {
					processSchedule(nextSchedule)
				})
			}
			time.Sleep(100 * time.Millisecond) // Sleep for a short duration to avoid busy waiting
		}
	}
}

func processSchedule(schedule models.Scheduler) {
	if isProcessed(schedule.ID) {
		return
	}

	log.Print("Processing schedule: ", schedule.ID)

	// Update status async
	go func(schedule models.Scheduler) {
		err := repository.UpdateSchedulerStatus(schedule, "in-progress")
		if err != nil {
			log.Printf("Error updating status: %v", err)
		}
	}(schedule)

	// Execute the webhook
	err := executeWebhook(schedule.WebhookURL, schedule.Payload, schedule)
	if err != nil {
		log.Printf("Error executing webhook for schedule ID %s: %v", schedule.ID, err)
		return
	}

	markProcessed(schedule)
}
