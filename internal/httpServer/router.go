package httpserver

import (
	"net/http"

	"github.com/Aspors/errhub-backend/internal/httpserver/handler"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	eventHandler := handler.NewEventHandler()

	mux.HandleFunc("GET /health", handler.Health)

	mux.HandleFunc("POST /api/events", eventHandler.Capture)
	
	return mux
}
