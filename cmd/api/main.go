package main

import (
	"log"

	"github.com/Aspors/errhub-backend/internal/config"
	"github.com/Aspors/errhub-backend/internal/httpServer"
)

func main() {
	cfg := config.LoadEnv()

	router := httpServer.NewRouter()
	server := httpServer.New(router, cfg.PORT)

	if err := server.Run(); err != nil{
		log.Fatal(err);
	}
}
