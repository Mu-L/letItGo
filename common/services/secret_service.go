package services

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func LiftENV() {
	envPath := "./.env"
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		log.Fatalf("Error: .env file does not exist at path: %s", envPath)
		return
	}

	if err := godotenv.Load(envPath); err != nil {
		log.Fatalf("Error loading environment variables: %v", err)
	} else {
		log.Println("Successfully loaded environment variables")
	}
}
