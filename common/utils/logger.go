package utils

import (
	"io"
	"log"
	"os"
)

func LoggerInit(path string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)
}
