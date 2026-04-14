package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/Aspors/errhub-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventHandler struct {
	db *pgxpool.Pool
}

func NewEventHandler(db *pgxpool.Pool) *EventHandler {
	return &EventHandler{
		db: db,
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

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal payload to JSON: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	query := `INSERT INTO events (project_id, payload) VALUES ($1, $2)`
	
	_, err = h.db.Exec(r.Context(), query, payload.ProjectID, payloadBytes)
	if err != nil {
		log.Printf("Database insert error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// TODO: Здесь будет деобфускация через MinIO и сохранение в PostgreSQL
	log.Printf("Received error from project [%s]: %s - %s", 
		payload.ProjectID, payload.Error.Type, payload.Error.Message)

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status": "captured"}`))
}
