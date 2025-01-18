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

	"github.com/Sumit189letItGo/database"
	"github.com/Sumit189letItGo/repository"
	"github.com/Sumit189letItGo/routes"
	"github.com/Sumit189letItGo/services"

	"github.com/gorilla/mux"
)

func main() {
	// Connect to MongoDB
	if err := database.Connect(); err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}

	// Initialize scheduler only after successful DB connection
	repository.InitializeSchedulerRepository()

	// Connect to Redis
	repository.RedisConnect()

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := &sync.WaitGroup{}

	// Start the polling service
	wg.Add(1)
	go func() {
		defer wg.Done()
		services.PollAndProduce(ctx)
	}()

	// Start the consumer service
	wg.Add(1)
	go func() {
		defer wg.Done()
		services.ConsumeAndProcess(ctx)
	}()

	// Router
	router := mux.NewRouter()
	routes.WebhookRoutes(router)

	// Create HTTP server
	server := &http.Server{
		Addr:    ":8081",
		Handler: router,
	}

	// Start HTTP server in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("Server running on port 8081")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	// Channel to listen for interrupt or terminate signals
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)

	// Block until a signal is received
	<-sigchan
	log.Println("Shutdown signal received")

	// Create a deadline to wait for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Attempt graceful shutdown of the HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
	log.Println("HTTP server stopped gracefully")

	// Cancel the main context to signal all goroutines to stop
	cancel()

	// Wait for all goroutines to finish
	wg.Wait()
	log.Println("All services stopped gracefully")
}
