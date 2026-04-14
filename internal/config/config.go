package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	PORT string
}

func LoadEnv() Config {
	if err := godotenv.Load(".env"); err != nil{
		log.Fatal(err);
	}

	port := os.Getenv("PORT");
	if(port == ""){
		port = "8080"
	}

	return Config{
		PORT: port,
	}
}
