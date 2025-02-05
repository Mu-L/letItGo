package services

import (
	"log"

	"github.com/joho/godotenv"
)

func LiftENV() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading environment variables: %v", err)
	} else {
		log.Println("Successfully loaded environment variables")
	}
}
