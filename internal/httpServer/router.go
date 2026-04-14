package httpServer

import (
	"net/http"

	"github.com/Aspors/errhub-backend/internal/httpServer/handler"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux();

	mux.HandleFunc("GET /health", handler.Health)

	return mux
}
