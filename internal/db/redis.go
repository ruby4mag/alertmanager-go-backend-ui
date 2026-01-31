package db

import (
	"os"

	"github.com/go-redis/redis/v8"
)

var RedisClient *redis.Client

func init() {
	redisURI := os.Getenv("REDIS_URI")
	if redisURI == "" {
		redisURI = "localhost:6379"
	}

	RedisClient = redis.NewClient(&redis.Options{
		Addr: redisURI,
	})
}
