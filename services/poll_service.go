package services

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Sumit189letItGo/models"
	"github.com/Sumit189letItGo/repository"
)

const (
	workerCount    = 6
	queueSize      = 100
	cacheWindow    = 10 * time.Minute
	fetchWindow    = 5 * time.Second
	maxFetchPerWin = 10
)

var (
	ctx       = context.Background()
	heapMutex sync.Mutex
)

func PollSchedules() {
	log.Println("Polling schedules")
	scheduleQueues := make([]chan models.Scheduler, workerCount)
	for i := range scheduleQueues {
		scheduleQueues[i] = make(chan models.Scheduler, queueSize)
	}

	// Start worker goroutines
	for i := 0; i < workerCount; i++ {
		go worker(scheduleQueues[i], i)
	}

	ticker := time.NewTicker(fetchWindow)
	defer ticker.Stop()

	var idx int
	for {
		select {
		case <-ticker.C:
			schedules, err := repository.FetchPending(int64(maxFetchPerWin))
			if err != nil {
				log.Printf("Error fetching schedules: %v", err)
				continue
			}
			for _, s := range schedules {
				scheduleQueues[idx] <- s
				idx = (idx + 1) % workerCount
			}
		}
	}
}

func worker(queue chan models.Scheduler, workerID int) {
	log.Println("Worker started:", workerID)
	for {
		schedule := <-queue
		time.AfterFunc(time.Until(*schedule.NextRunTime), func() {
			processSchedule(schedule)
		})
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
	err := executeWebhook(schedule.ID)
	if err != nil {
		log.Printf("Error executing webhook for schedule ID %s: %v", schedule.ID, err)
		return
	} else {
		markProcessed(schedule)
	}
}
