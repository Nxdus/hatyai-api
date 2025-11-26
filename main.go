package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Nxdus/hatyai-api/routes"
	"github.com/Nxdus/hatyai-api/services"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

func main() {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
		DB:   0,
	})
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Redis connection failed: %v", err)
	} else {
		log.Printf("Redis connected: %s", redisAddr)
	}

	fetcher := services.NewHTTPFetcher()
	sosService := services.NewRedisSOSService(rdb, fetcher)

	go func() {
		if _, err := sosService.GetRaw(); err != nil {
			log.Printf("Cache warm-up failed: %v", err)
		} else {
			log.Printf("Cache warm-up completed")
		}
	}()

	routes.RegisterRoutes(app, sosService, rdb)

	log.Fatal(app.Listen(":3000"))
}
