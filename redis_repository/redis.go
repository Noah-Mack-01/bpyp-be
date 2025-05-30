package redis_repository

import (
	"context"
	"os"

	redis "github.com/redis/go-redis/v9"
)

var ctx = context.Background()

// Global read-only context variable
var CachedExercises = GetContext()

func GetClient() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "redis-10347.c240.us-east-1-3.ec2.redns.redis-cloud.com:10347",
		Password: os.Getenv("REDIS_PW"), // no password set
		DB:       0,                     // use default DB
	})
	return rdb
}

func GetContext() *ExerciseContext {
	client := GetClient()

	// Get all members from the attributes set
	attributes, err := client.SMembers(ctx, "attributes").Result()
	if err != nil {
		attributes = []string{} // Return empty slice on error
	}

	// Get all members from the exercises set
	exercises, err := client.SMembers(ctx, "exercises").Result()
	if err != nil {
		exercises = []string{} // Return empty slice on error
	}

	return &ExerciseContext{
		Exercises:  exercises,
		Attributes: attributes,
	}
}
