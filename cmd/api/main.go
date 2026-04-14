package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Aspors/errhub-backend/internal/config"
	"github.com/Aspors/errhub-backend/internal/httpserver"
	"github.com/Aspors/errhub-backend/internal/storage/postgres"
	"github.com/Aspors/errhub-backend/internal/storage/redis"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()  

	cfg := config.LoadEnv()

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dbCancel()

	db, err := postgres.New(dbCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer db.Close()
	
	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Println("Successfully connected to PostgreSQL!")

	rdb, err := redis.New(dbCtx, cfg.RedisAddr, cfg.RedisPass)
	if err != nil {
		log.Fatalf("Redis initialization failed: %v", err)
	}
	defer rdb.Close()

	log.Println("Successfully connected to Redis!")

	router := httpserver.NewRouter(db.Pool, rdb)
	server := httpserver.New(router, cfg.Port)

	if err := server.Run(ctx); err != nil{
		log.Fatal(err)
	}
}
