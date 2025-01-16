package main

import (
	"log"
	"net/http"

	"github.com/Sumit189letItGo/database"
	"github.com/Sumit189letItGo/repository"
	"github.com/Sumit189letItGo/routes"
	"github.com/Sumit189letItGo/services"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func main() {
	// Connect to MongoDB
	if err := database.Connect(); err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}

	// Initialize scheduler only after successful DB connection
	repository.InitializeSchedulerRepository()

	// Connect to Redis
	repository.RedisConnect()

	// Start the polling service in a goroutine
	go services.PollSchedules()

	// Router
	router := mux.NewRouter()
	routes.WebhookRoutes(router)

	// Start server
	log.Println("Server running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
