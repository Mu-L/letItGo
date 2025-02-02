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
	common_services "github.com/Sumit189/letItGo/common/services"
	"github.com/Sumit189/letItGo/common/utils"
	"github.com/Sumit189/letItGo/consumer/services"
)

func main() {
	utils.LoggerInit("consumer.log")
	consumerAsciiArt := `
	####     ### ###  #### ##    ####   #### ##   ## ##    ## ##   
	 ##       ##  ##  # ## ##     ##    # ## ##  ##   ##  ##   ##  
	 ##       ##        ##        ##      ##     ##       ##   ##  
	 ##       ## ##     ##        ##      ##     ##  ###  ##   ##  
	 ##       ##        ##        ##      ##     ##   ##  ##   ##  
	 ##  ##   ##  ##    ##        ##      ##     ##   ##  ##   ##  
	### ###  ### ###   ####      ####    ####     ## ##    ## ##   
																   
	 ## ##    ## ##   ###  ##   ## ##   ##  ###  ##   ##  ### ###  ### ##   
	##   ##  ##   ##    ## ##  ##   ##  ##   ##   ## ##    ##  ##   ##  ##  
	##       ##   ##   # ## #  ####     ##   ##  # ### #   ##       ##  ##  
	##       ##   ##   ## ##    #####   ##   ##  ## # ##   ## ##    ## ##   
	##       ##   ##   ##  ##      ###  ##   ##  ##   ##   ##       ## ##   
	##   ##  ##   ##   ##  ##  ##   ##  ##   ##  ##   ##   ##  ##   ##  ##  
	 ## ##    ## ##   ###  ##   ## ##    ## ##   ##   ##  ### ###  #### ##  
	`
	log.Println(consumerAsciiArt)

	common_services.LiftENV()
	utils.AESInit()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to MongoDB
	if err := database.Connect(ctx); err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}

	// Initialize scheduler only after successful DB connection
	repository.InitializeSchedulerRepository()
	repository.InitializeArchiveRepository()

	// Connect to Redis
	repository.RedisConnect(ctx)

	wg := &sync.WaitGroup{}

	// Start the consumer service
	wg.Add(1)
	go func() {
		defer wg.Done()
		services.ConsumeAndProcess(ctx)
	}()

	// Channel to listen for interrupt or terminate signals
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)

	// Block until a signal is received
	<-sigchan
	log.Println("Shutdown signal received")

	// Wait for all goroutines to finish
	wg.Wait()
	log.Println("All services stopped gracefully")
}
