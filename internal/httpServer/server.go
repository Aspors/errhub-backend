package httpserver

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"
)

type Server struct {
	httpServer *http.Server
}

func New(handler http.Handler, port string) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:         ":" + port,
			Handler:      handler,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

func (s *Server) Run(ctx context.Context) error {
	serverError := make(chan error, 1)

	go func() {
		log.Printf("Server started on %s", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	select {
	case err := <-serverError:
		return err
	case <-ctx.Done():
		log.Println("Shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return err
	}

	log.Println("Server gracefully stopped")
	return nil
}
