package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string 
	RedisAddr   string 
	RedisPass   string 
}

func LoadEnv() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("Info: No .env file found. Using OS environment variables.")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbUser := os.Getenv("POSTGRES_USER")
	dbPass := os.Getenv("POSTGRES_PASSWORD")
	dbName := os.Getenv("POSTGRES_DB")
	dbHost := os.Getenv("POSTGRES_HOST")
	
	if dbHost == "" {
		dbHost = "localhost"
	}

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", dbUser, dbPass, dbHost, dbName)

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisPass := os.Getenv("REDIS_PASSWORD")

	return &Config{
		Port:        port,
		DatabaseURL: dbURL,
		RedisAddr:   redisAddr,
		RedisPass:   redisPass,
	}
}
