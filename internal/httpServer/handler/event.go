package handler

import (
	"net/http"

	"github.com/Aspors/errhub-backend/internal/models"
	eventsvc "github.com/Aspors/errhub-backend/internal/service/event"
	hash "github.com/Aspors/errhub-backend/internal/utils"
)

type EventHandler struct {
	processor *eventsvc.Processor
}

func NewEventHandler(processor *eventsvc.Processor) *EventHandler {
	return &EventHandler{processor: processor}
}

// CaptureResponse is returned after a successful event ingestion.
type CaptureResponse struct {
	Status string `json:"status" example:"captured"`
}

// Capture godoc
//
//	@Summary     Capture error event
//	@Description Accepts an error event from the errhub-package SDK. The payload is validated immediately and processed asynchronously by a worker pool. No authentication required — the project_id in the payload must match an existing project UUID.
//	@Tags        events
//	@Accept      json
//	@Produce     json
//	@Param       body body     models.EventPayload true "Error event payload"
//	@Success     202  {object} CaptureResponse
//	@Failure     400  {object} ErrorResponse
//	@Failure     422  {object} ErrorResponse "validation error"
//	@Router      /api/events [post]
func (h *EventHandler) Capture(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var payload models.EventPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := payload.Validate(); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	fingerprint := hash.GenerateIssueHash(payload.ProjectID, payload.Error.Type, payload.Error.Message)

	h.processor.Enqueue(eventsvc.Job{
		Payload:     payload,
		Fingerprint: fingerprint,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"captured"}`))
}
