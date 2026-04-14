package httpserver

import (
	"net/http"

	"github.com/Aspors/errhub-backend/internal/httpserver/handler"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func NewRouter(db *pgxpool.Pool, rdb *redis.Client) http.Handler {
	mux := http.NewServeMux()

	eventHandler := handler.NewEventHandler(db, rdb)

	mux.HandleFunc("GET /health", handler.Health)

	mux.HandleFunc("POST /api/events", eventHandler.Capture)
	
	return corsMiddleware(mux)
}
