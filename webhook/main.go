package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Sumit189/letItGo/common/database"
	"github.com/Sumit189/letItGo/common/repository"
	"github.com/Sumit189/letItGo/webhook/routes"

	"github.com/gorilla/mux"
)

func main() {
	webhookAsciiArt := `
	####     ### ###  #### ##    ####   #### ##   ## ##    ## ##   
	 ##       ##  ##  # ## ##     ##    # ## ##  ##   ##  ##   ##  
	 ##       ##        ##        ##      ##     ##       ##   ##  
	 ##       ## ##     ##        ##      ##     ##  ###  ##   ##  
	 ##       ##        ##        ##      ##     ##   ##  ##   ##  
	 ##  ##   ##  ##    ##        ##      ##     ##   ##  ##   ##  
	### ###  ### ###   ####      ####    ####     ## ##    ## ##   
																   
	  ##     ### ##     ####   
	   ##     ##  ##     ##    
	 ## ##    ##  ##     ##    
	 ##  ##   ##  ##     ##    
	 ## ###   ## ##      ##    
	 ##  ##   ##         ##    
	###  ##  ####       ####   
	`
	log.Println(webhookAsciiArt)
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

	// Router
	router := mux.NewRouter()
	routes.WebhookRoutes(router)

	// Create and start HTTP server
	server := &http.Server{
		Addr:    ":8081",
		Handler: router,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("Server running on port 8081")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	// Handle shutdown signals
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
	<-sigchan
	log.Println("Shutdown signal received")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
	log.Println("HTTP server stopped gracefully")

	cancel()
	wg.Wait()
	log.Println("All services stopped gracefully")
}
