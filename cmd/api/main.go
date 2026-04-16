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
	"github.com/Aspors/errhub-backend/internal/service/cleanup"
	eventsvc "github.com/Aspors/errhub-backend/internal/service/event"
	"github.com/Aspors/errhub-backend/internal/service/sourcemap"
	"github.com/Aspors/errhub-backend/internal/storage/postgres"
	"github.com/Aspors/errhub-backend/internal/storage/redis"
	"github.com/Aspors/errhub-backend/internal/storage/s3"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.LoadEnv()

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dbCancel()
	db, err := postgres.New(dbCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database initialization failed: %v", err)
	}
	defer db.Close()

	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	if cfg.SeedDemo {
		if err := db.RunSeedMigrations(cfg.DatabaseURL); err != nil {
			log.Fatalf("failed to run seed migrations: %v", err)
		}
	}

	log.Println("connected to PostgreSQL")

	redisCtx, redisCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer redisCancel()
	rdb, err := redis.New(redisCtx, cfg.RedisAddr, cfg.RedisPass)
	if err != nil {
		log.Fatalf("redis initialization failed: %v", err)
	}
	defer rdb.Close()
	log.Println("connected to Redis")

	// MinIO for source map storage.
	var storage *s3.Storage
	var srcSvc *sourcemap.Service
	if cfg.MinioEndpoint != "" {
		minioCtx, minioCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer minioCancel()
		storage, err = s3.New(minioCtx, cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioBucket)
		if err != nil {
			log.Fatalf("minio initialization failed: %v", err)
		}
		log.Println("connected to MinIO")

		srcSvc = sourcemap.New(storage, db.Pool)

		// Start daily cleanup of stale source maps.
		cleanup.Start(ctx, db.Pool, storage)
	} else {
		log.Println("MINIO_ENDPOINT not set; source map features disabled")
	}

	// Start async event processor: 500-event buffer, 4 worker goroutines.
	processor := eventsvc.NewProcessor(db.Pool, rdb, srcSvc, 500)
	processor.Start(4, 100, time.Second)
	defer processor.Stop() // drains the queue before exit

	router := httpserver.NewRouter(db.Pool, rdb, processor, storage, srcSvc, cfg.JWTSecret, cfg.AdminKey, cfg.CORSOrigins)
	server := httpserver.New(router, cfg.Port)

	if err := server.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
