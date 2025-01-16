package services

import (
	"container/heap"
	"context"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/Sumit189letItGo/models"
	"github.com/Sumit189letItGo/repository"
)

const (
	workerCount    = 6
	queueSize      = 100
	cacheWindow    = 1 * time.Minute
	fetchWindow    = 5 * time.Second
	maxFetchPerWin = 10
	poolInterval   = 10 * time.Second
	workerInterval = 1 * time.Second
)

var (
	ctx          = context.Background()
	scheduleHeap = &ScheduleHeap{}
	heapMutex    sync.Mutex
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
	scheduleQueues := make([]chan models.Scheduler, workerCount)
	for i := range scheduleQueues {
		scheduleQueues[i] = make(chan models.Scheduler, queueSize)
	}

	// Start worker goroutines
	for i := 0; i < workerCount; i++ {
		time.Sleep(workerInterval)
		go worker(scheduleQueues[i], i)
	}

	select {}
}

func worker(queue chan models.Scheduler, workerID int) {
	log.Println("Worker started: ", workerID)
	heap.Init(scheduleHeap)

	for {
		select {
		case schedule := <-queue:
			heapMutex.Lock()
			heap.Push(scheduleHeap, schedule)
			heapMutex.Unlock()
		default:
			heapMutex.Lock()
			if scheduleHeap.Len() > 0 {
				nextSchedule := heap.Pop(scheduleHeap).(models.Scheduler)
				nextRunTime := nextSchedule.NextRunTime
				// Schedule the processing of the next schedule at its next run time
				time.AfterFunc(time.Until(*nextRunTime), func() {
					processSchedule(nextSchedule)
				})
			}
			heapMutex.Unlock()
			fetchAndProcessSchedules(workerID)
			time.Sleep(poolInterval)
		}
	}
}

func fetchAndProcessSchedules(workerID int) {
	limit := int64(10)
	schedules, err := repository.FetchPending(limit)
	log.Printf("Worker %d fetched %d schedules", workerID, len(schedules))
	if err != nil {
		log.Printf("Error fetching schedules for worker %d: %v", workerID, err)
		return
	}

	sort.Slice(schedules, func(i, j int) bool {
		return schedules[i].NextRunTime.Before(*schedules[j].NextRunTime)
	})

	if len(schedules) > 0 {
		var idsToAdd []interface{}
		for _, task := range schedules {
			idsToAdd = append(idsToAdd, task.ID)
		}
		pipe := repository.RedisClient.TxPipeline()
		pipe.SAdd(context.Background(), "in_queue", idsToAdd...)
		pipe.Expire(context.Background(), "in_queue", 1*time.Minute)
		_, err = pipe.Exec(context.Background())
		if err != nil {
			return
		}
	}

	heapMutex.Lock()
	defer heapMutex.Unlock()
	for _, schedule := range schedules {
		if isProcessed(schedule.ID) {
			continue
		}
		heap.Push(scheduleHeap, schedule)
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
	err := executeWebhook(schedule)
	if err != nil {
		log.Printf("Error executing webhook for schedule ID %s: %v", schedule.ID, err)
		return
	}

	markProcessed(schedule)
}
