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

	router := httpserver.NewRouter(db.Pool)
	server := httpserver.New(router, cfg.Port)

	if err := server.Run(ctx); err != nil{
		log.Fatal(err)
	}
}
