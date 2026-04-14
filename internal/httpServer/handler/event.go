package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/Aspors/errhub-backend/internal/models"
)

type EventHandler struct {
	//TODO: Позже здесь появится db *pgxpool.Pool
}

func NewEventHandler() *EventHandler {
	return &EventHandler{}
}

func (h *EventHandler) Capture(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var payload models.EventPayload
	
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("Failed to decode payload: %v", err)
		http.Error(w, "Bad Request: invalid JSON", http.StatusBadRequest)
		return
	}

	// TODO: Здесь будет деобфускация через MinIO и сохранение в PostgreSQL
	log.Printf("Received error from project [%s]: %s - %s", 
		payload.ProjectID, payload.Error.Type, payload.Error.Message)

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status": "captured"}`))
}
