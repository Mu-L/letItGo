package repository

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ctx         = context.Background()
	RedisClient *redis.Client
)

func RedisConnect() {
	redisAddrss := os.Getenv("REDIS_ADDRESS")
	if redisAddrss == "" {
		log.Fatal("REDIS_ADDRESS not set in environment")
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB := os.Getenv("REDIS_DB")
	db := 0
	if redisDB != "" {
		db, _ = strconv.Atoi(redisDB)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddrss,
		Password: redisPassword,
		DB:       db,
	})

	if err := RedisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	} else {
		log.Println("Connected to Redis")
		// clear all keys
		RedisClient.FlushAll(ctx)
	}
}
