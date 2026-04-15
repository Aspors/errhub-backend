package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	RedisAddr   string
	RedisPass   string
	JWTSecret   string
	AdminKey    string
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	CORSOrigins    []string
}

func LoadEnv() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("Info: No .env file found. Using OS environment variables.")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbHost := os.Getenv("POSTGRES_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		dbHost,
		os.Getenv("POSTGRES_DB"),
	)

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "change-me-in-production"
		log.Println("WARNING: JWT_SECRET is not set; using insecure default")
	}

	adminKey := os.Getenv("ADMIN_KEY")
	if adminKey == "" {
		log.Println("WARNING: ADMIN_KEY is not set; POST /api/admin/users will be disabled")
	}

	minioBucket := os.Getenv("MINIO_BUCKET")
	if minioBucket == "" {
		minioBucket = "sourcemaps"
	}

	var corsOrigins []string
	if raw := os.Getenv("CORS_ORIGINS"); raw != "" {
		for _, o := range strings.Split(raw, ",") {
			if o = strings.TrimSpace(o); o != "" {
				corsOrigins = append(corsOrigins, o)
			}
		}
	}

	return &Config{
		Port:           port,
		DatabaseURL:    dbURL,
		RedisAddr:      redisAddr,
		RedisPass:      os.Getenv("REDIS_PASSWORD"),
		JWTSecret:      jwtSecret,
		AdminKey:       adminKey,
		MinioEndpoint:  os.Getenv("MINIO_ENDPOINT"),
		MinioAccessKey: os.Getenv("MINIO_USER"),
		MinioSecretKey: os.Getenv("MINIO_PASSWORD"),
		MinioBucket:    minioBucket,
		CORSOrigins:    corsOrigins,
	}
}
