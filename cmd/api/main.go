package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Aspors/errhub-backend/internal/config"
	"github.com/Aspors/errhub-backend/internal/httpserver"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()  

	cfg := config.LoadEnv()

	router := httpserver.NewRouter()
	server := httpserver.New(router, cfg.PORT)

	if err := server.Run(ctx); err != nil{
		log.Fatal(err)
	}
}
