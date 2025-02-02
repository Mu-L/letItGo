package services

import (
	"log"

	"github.com/joho/godotenv"
)

func LiftENV() {
	if err := godotenv.Load("../../.env"); err != nil {
		log.Fatal("Error loading environment variables")
	} else {
		log.Println("Successfully loaded environment variables")
	}
}
