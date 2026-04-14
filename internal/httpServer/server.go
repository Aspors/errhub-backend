package httpServer

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Server struct {
	httpServer *http.Server
}

func New(handler http.Handler, port string) *Server{
	return  &Server{
		httpServer: &http.Server{
			Addr: ":" + port,
			Handler: handler,
		},
	}
}

func (s *Server) Run() error {
	go func() {
		log.Println("server started on", s.httpServer.Addr)

		if err := s.httpServer.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)

	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	log.Println("Shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return  err;
	}

	log.Println("Server stopped")

	return nil
}
