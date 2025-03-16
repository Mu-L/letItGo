package services

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func LiftENV() {
	paths := []string{
		".env",                   // Current directory
		"../.env",                // Parent directory
		"/app/.env",              // Root app directory in Docker
		filepath.Dir(os.Args[0]), // Binary directory
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			err := godotenv.Load(path)
			if err != nil {
				log.Println("Error loading env from:", path, err)
			} else {
				log.Println("Loaded environment from:", path)
				return
			}
		}
	}

	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading environment variables:", err)
		log.Println("Continuing with environment variables already set in the system...")
	}
}
