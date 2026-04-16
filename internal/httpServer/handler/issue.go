package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/Aspors/errhub-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IssueHandler struct {
	db *pgxpool.Pool
}

func NewIssueHandler(db *pgxpool.Pool) *IssueHandler {
	return &IssueHandler{db: db}
}

// IssueListResponse wraps a paginated list of issues.
type IssueListResponse struct {
	Items  []models.Issue `json:"items"`
	Total  int64          `json:"total"  example:"143"`
	Limit  int            `json:"limit"  example:"50"`
	Offset int            `json:"offset" example:"0"`
}

// IssueEventRow is a single raw event occurrence attached to an issue.
// ResolvedStack is non-nil when the stack trace has been deobfuscated; the
// frontend should prefer it over the minified stacktrace inside Payload.
type IssueEventRow struct {
	Payload       json.RawMessage `json:"payload"`
	ResolvedStack *string         `json:"resolved_stack"`
	CreatedAt     string          `json:"created_at" example:"2024-01-15T10:30:00Z"`
}

// IssueDetailResponse combines the issue with its recent event occurrences.
type IssueDetailResponse struct {
	Issue  models.Issue    `json:"issue"`
	Events []IssueEventRow `json:"events"`
}

// UpdateStatusRequest is the body for PATCH /api/projects/{projectId}/issues/{issueId}.
type UpdateStatusRequest struct {
	Status string `json:"status" example:"resolved" enums:"open,resolved,ignored"`
}

// List godoc
//
//	@Summary      List issues
//	@Description  Returns a paginated list of deduplicated issues for a project, ordered by last_seen descending. Each item includes the occurrence counter so the frontend can show "occurred N times".
//	@Tags         issues
//	@Produce      json
//	@Security     BearerAuth
//	@Param        projectId path     string true  "Project UUID"
//	@Param        limit     query    int    false "Max results (default 50, max 200)"
//	@Param        offset    query    int    false "Pagination offset (default 0)"
//	@Success      200       {object} IssueListResponse
//	@Failure      401       {object} ErrorResponse
//	@Failure      500       {object} ErrorResponse
//	@Router       /api/projects/{projectId}/issues [get]
func (h *IssueHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	limit, offset := parsePagination(r, 50, 200)

	// Run count and data queries in parallel to avoid COUNT(*) OVER() full scan.
	type countResult struct {
		n   int64
		err error
	}
	countCh := make(chan countResult, 1)
	go func() {
		var n int64
		err := h.db.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM issues WHERE project_id = $1`, projectID).Scan(&n)
		countCh <- countResult{n, err}
	}()

	rows, err := h.db.Query(r.Context(), `
		SELECT id, project_id, fingerprint, level, error_type, error_message,
		       occurrences, first_seen, last_seen, status
		FROM issues
		WHERE project_id = $1
		ORDER BY last_seen DESC
		LIMIT $2 OFFSET $3`,
		projectID, limit, offset)
	if err != nil {
		log.Printf("failed to list issues [project=%s]: %v", projectID, err)
		writeError(w, http.StatusInternalServerError, "failed to fetch issues")
		return
	}
	defer rows.Close()

	issues := make([]models.Issue, 0)
	for rows.Next() {
		var issue models.Issue
		if err := rows.Scan(
			&issue.ID, &issue.ProjectID, &issue.Fingerprint,
			&issue.Level, &issue.ErrorType, &issue.ErrorMessage,
			&issue.Occurrences, &issue.FirstSeen, &issue.LastSeen, &issue.Status,
		); err != nil {
			log.Printf("failed to scan issue row: %v", err)
			continue
		}
		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		log.Printf("failed to iterate issue rows [project=%s]: %v", projectID, err)
		writeError(w, http.StatusInternalServerError, "failed to fetch issues")
		return
	}

	cr := <-countCh
	if cr.err != nil {
		log.Printf("failed to count issues [project=%s]: %v", projectID, cr.err)
		writeError(w, http.StatusInternalServerError, "failed to fetch issues")
		return
	}

	writeJSON(w, http.StatusOK, IssueListResponse{
		Items:  issues,
		Total:  cr.n,
		Limit:  limit,
		Offset: offset,
	})
}

// Get godoc
//
//	@Summary      Get issue
//	@Description  Returns issue details and the last 10 raw event payloads that caused it.
//	@Tags         issues
//	@Produce      json
//	@Security     BearerAuth
//	@Param        projectId path     string true "Project UUID"
//	@Param        issueId   path     string true "Issue UUID"
//	@Success      200       {object} IssueDetailResponse
//	@Failure      401       {object} ErrorResponse
//	@Failure      404       {object} ErrorResponse
//	@Router       /api/projects/{projectId}/issues/{issueId} [get]
func (h *IssueHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	issueID := r.PathValue("issueId")

	var issue models.Issue
	err := h.db.QueryRow(r.Context(), `
		SELECT id, project_id, fingerprint, level, error_type, error_message,
		       occurrences, first_seen, last_seen, status
		FROM issues WHERE id = $1 AND project_id = $2`,
		issueID, projectID).Scan(
		&issue.ID, &issue.ProjectID, &issue.Fingerprint,
		&issue.Level, &issue.ErrorType, &issue.ErrorMessage,
		&issue.Occurrences, &issue.FirstSeen, &issue.LastSeen, &issue.Status,
	)
	if err != nil {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}

	rows, err := h.db.Query(r.Context(), `
		SELECT payload, created_at, resolved_stack FROM events
		WHERE issue_id = $1 ORDER BY created_at DESC LIMIT 10`,
		issueID)
	if err != nil {
		log.Printf("failed to fetch events for issue [id=%s]: %v", issueID, err)
		writeError(w, http.StatusInternalServerError, "failed to fetch events")
		return
	}
	defer rows.Close()

	events := make([]IssueEventRow, 0)
	for rows.Next() {
		var raw json.RawMessage
		var createdAt time.Time
		var resolvedStack *string
		if err := rows.Scan(&raw, &createdAt, &resolvedStack); err != nil {
			log.Printf("failed to scan event row: %v", err)
			continue
		}
		events = append(events, IssueEventRow{
			Payload:       raw,
			ResolvedStack: resolvedStack,
			CreatedAt:     createdAt.UTC().Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("failed to iterate event rows [issue=%s]: %v", issueID, err)
		writeError(w, http.StatusInternalServerError, "failed to fetch events")
		return
	}

	writeJSON(w, http.StatusOK, IssueDetailResponse{Issue: issue, Events: events})
}

// UpdateStatus godoc
//
//	@Summary      Update issue status
//	@Description  Changes the status of an issue. Use "resolved" to mark it fixed, "ignored" to suppress it, or "open" to reopen.
//	@Tags         issues
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        projectId path     string              true "Project UUID"
//	@Param        issueId   path     string              true "Issue UUID"
//	@Param        body      body     UpdateStatusRequest true "New status"
//	@Success      200       {object} models.Issue
//	@Failure      400       {object} ErrorResponse
//	@Failure      401       {object} ErrorResponse
//	@Failure      404       {object} ErrorResponse
//	@Failure      422       {object} ErrorResponse
//	@Router       /api/projects/{projectId}/issues/{issueId} [patch]
func (h *IssueHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	issueID := r.PathValue("issueId")

	var req UpdateStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	validStatuses := map[string]struct{}{"open": {}, "resolved": {}, "ignored": {}}
	if _, ok := validStatuses[req.Status]; !ok {
		writeError(w, http.StatusUnprocessableEntity, "status must be one of: open, resolved, ignored")
		return
	}

	var issue models.Issue
	err := h.db.QueryRow(r.Context(), `
		UPDATE issues SET status = $1
		WHERE id = $2 AND project_id = $3
		RETURNING id, project_id, fingerprint, level, error_type, error_message,
		          occurrences, first_seen, last_seen, status`,
		req.Status, issueID, projectID).Scan(
		&issue.ID, &issue.ProjectID, &issue.Fingerprint,
		&issue.Level, &issue.ErrorType, &issue.ErrorMessage,
		&issue.Occurrences, &issue.FirstSeen, &issue.LastSeen, &issue.Status,
	)
	if err != nil {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}

	writeJSON(w, http.StatusOK, issue)
}

func parsePagination(r *http.Request, defaultLimit, maxLimit int) (limit, offset int) {
	limit = defaultLimit
	offset = 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= maxLimit {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return
}
