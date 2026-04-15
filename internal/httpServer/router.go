package httpserver

import (
	"net/http"

	_ "github.com/Aspors/errhub-backend/docs"
	"github.com/Aspors/errhub-backend/internal/httpserver/handler"
	"github.com/Aspors/errhub-backend/internal/httpserver/middleware"
	eventsvc "github.com/Aspors/errhub-backend/internal/service/event"
	"github.com/Aspors/errhub-backend/internal/storage/s3"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, PATCH, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Authorization, X-Admin-Key")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func NewRouter(
	db *pgxpool.Pool,
	rdb *redis.Client,
	processor *eventsvc.Processor,
	storage *s3.Storage,
	jwtSecret, adminKey string,
) http.Handler {
	mux := http.NewServeMux()

	protect := middleware.JWTAuth(jwtSecret)

	eventHandler := handler.NewEventHandler(processor)
	authHandler := handler.NewAuthHandler(db, jwtSecret)
	adminHandler := handler.NewAdminHandler(db, adminKey)
	projectHandler := handler.NewProjectHandler(db)
	issueHandler := handler.NewIssueHandler(db)
	statsHandler := handler.NewStatsHandler(db)
	sourcemapHandler := handler.NewSourcemapHandler(db, storage)

	// Swagger UI — available at /swagger/index.html
	mux.Handle("/swagger/", httpSwagger.WrapHandler)

	// Public
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)

	// Admin — requires X-Admin-Key header
	mux.HandleFunc("POST /api/admin/users", adminHandler.CreateUser)

	// Event ingestion — public, SDK does not send auth headers
	mux.HandleFunc("POST /api/events", eventHandler.Capture)

	// Projects (protected)
	mux.HandleFunc("POST /api/projects", protect(projectHandler.Create))
	mux.HandleFunc("GET /api/projects", protect(projectHandler.List))
	mux.HandleFunc("GET /api/projects/{projectId}", protect(projectHandler.Get))
	mux.HandleFunc("DELETE /api/projects/{projectId}", protect(projectHandler.Delete))

	// Stats (protected)
	mux.HandleFunc("GET /api/projects/{projectId}/stats", protect(statsHandler.Get))

	// Issues (protected)
	mux.HandleFunc("GET /api/projects/{projectId}/issues", protect(issueHandler.List))
	mux.HandleFunc("GET /api/projects/{projectId}/issues/{issueId}", protect(issueHandler.Get))
	mux.HandleFunc("PATCH /api/projects/{projectId}/issues/{issueId}", protect(issueHandler.UpdateStatus))

	// Source maps (protected) — called from CI after each build
	mux.HandleFunc("POST /api/projects/{projectId}/releases/{release}/sourcemaps", protect(sourcemapHandler.Upload))

	return corsMiddleware(mux)
}
