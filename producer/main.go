package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Sumit189/letItGo/common/database"
	"github.com/Sumit189/letItGo/common/repository"
	"github.com/Sumit189/letItGo/producer/services"
)

func main() {
	producerAsciiArt := `
	####     ### ###  #### ##    ####   #### ##   ## ##    ## ##   
	 ##       ##  ##  # ## ##     ##    # ## ##  ##   ##  ##   ##  
	 ##       ##        ##        ##      ##     ##       ##   ##  
	 ##       ## ##     ##        ##      ##     ##  ###  ##   ##  
	 ##       ##        ##        ##      ##     ##   ##  ##   ##  
	 ##  ##   ##  ##    ##        ##      ##     ##   ##  ##   ##  
	### ###  ### ###   ####      ####    ####     ## ##    ## ##   
																   
	### ##   ### ##    ## ##   ### ##   ##  ###   ## ##   ### ###  ### ##   
	 ##  ##   ##  ##  ##   ##   ##  ##  ##   ##  ##   ##   ##  ##   ##  ##  
	 ##  ##   ##  ##  ##   ##   ##  ##  ##   ##  ##        ##       ##  ##  
	 ##  ##   ## ##   ##   ##   ##  ##  ##   ##  ##        ## ##    ## ##   
	 ## ##    ## ##   ##   ##   ##  ##  ##   ##  ##        ##       ## ##   
	 ##       ##  ##  ##   ##   ##  ##  ##   ##  ##   ##   ##  ##   ##  ##  
	####     #### ##   ## ##   ### ##    ## ##    ## ##   ### ###  #### ##  
	`
	log.Println(producerAsciiArt)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to MongoDB
	if err := database.Connect(ctx); err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}

	// Initialize scheduler and connect to Redis
	repository.InitializeSchedulerRepository()
	repository.RedisConnect(ctx)

	wg := &sync.WaitGroup{}

	// Start the polling service
	wg.Add(1)
	go func() {
		defer wg.Done()
		services.PollAndProduce(ctx)
	}()

	// Handle shutdown signals
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
	<-sigchan
	log.Println("Shutdown signal received")

	// Cancel the main context to signal all goroutines to stop
	cancel()

	// Wait for all goroutines to finish
	wg.Wait()
	log.Println("All services stopped gracefully")
}
