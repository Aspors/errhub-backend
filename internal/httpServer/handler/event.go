package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/Aspors/errhub-backend/internal/models"
	hash "github.com/Aspors/errhub-backend/internal/utils"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type EventHandler struct {
	db *pgxpool.Pool
	redis *redis.Client
}

func NewEventHandler(db *pgxpool.Pool, rdb *redis.Client) *EventHandler {
	return &EventHandler{
		db: db,
		redis: rdb,
	}
}

func (h *EventHandler) Capture(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var payload models.EventPayload
	
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("Failed to decode payload: %v", err)
		http.Error(w, "Bad Request: invalid JSON", http.StatusBadRequest)
		return
	}

	issueHash := hash.GenerateIssueHash(payload.ProjectID, payload.Error.Type, payload.Error.Message)

	redisKey := fmt.Sprintf("issue:%s:count", issueHash)

	count, err := h.redis.Incr(r.Context(), redisKey).Result()
	if err != nil {
		log.Printf("Redis error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if count == 1 {
		payloadBytes, _ := json.Marshal(payload)
		
		query := `INSERT INTO events (project_id, payload) VALUES ($1, $2)`
		if _, err := h.db.Exec(r.Context(), query, payload.ProjectID, payloadBytes); err != nil {
			log.Printf("DB insert error: %v", err)
		}
		log.Printf("🔥 NEW ISSUE [%s]: Saved to Postgres. Hash: %s", payload.ProjectID, issueHash)
		
	} else {
		log.Printf("🛡️ DEDUPLICATED [%s]: Redis blocked duplicate. Hash: %s, Total hits: %d", payload.ProjectID, issueHash, count)
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status": "captured"}`))
}
