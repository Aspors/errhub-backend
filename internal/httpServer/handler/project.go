package handler

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"

	"github.com/Aspors/errhub-backend/internal/httpserver/middleware"
	"github.com/Aspors/errhub-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectHandler struct {
	db *pgxpool.Pool
}

func NewProjectHandler(db *pgxpool.Pool) *ProjectHandler {
	return &ProjectHandler{db: db}
}

// CreateProjectRequest is the body for POST /api/projects.
type CreateProjectRequest struct {
	Name string `json:"name" example:"My React App"`
}

// Create godoc
//
//	@Summary      Create project
//	@Description  Creates a new project and generates an API key. The returned `id` is what the errhub-package SDK uses as `projectId`.
//	@Tags         projects
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        body body     CreateProjectRequest true "Project details"
//	@Success      201  {object} models.Project
//	@Failure      400  {object} ErrorResponse
//	@Failure      401  {object} ErrorResponse
//	@Failure      422  {object} ErrorResponse
//	@Router       /api/projects [post]
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	createdBy, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateProjectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "name is required")
		return
	}

	apiKey, err := generateAPIKey()
	if err != nil {
		log.Printf("failed to generate api key: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	var project models.Project
	query := `
		INSERT INTO projects (name, api_key, user_id)
		VALUES ($1, $2, $3)
		RETURNING id, name, api_key, created_at`
	err = h.db.QueryRow(r.Context(), query, req.Name, apiKey, createdBy).
		Scan(&project.ID, &project.Name, &project.APIKey, &project.CreatedAt)
	if err != nil {
		log.Printf("failed to create project: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	writeJSON(w, http.StatusCreated, project)
}

// List godoc
//
//	@Summary      List projects
//	@Description  Returns all projects visible to authenticated users (company-wide).
//	@Tags         projects
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200 {array}  models.Project
//	@Failure      401 {object} ErrorResponse
//	@Router       /api/projects [get]
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	query := `SELECT id, name, api_key, created_at FROM projects ORDER BY created_at DESC`
	rows, err := h.db.Query(r.Context(), query)
	if err != nil {
		log.Printf("failed to list projects: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch projects")
		return
	}
	defer rows.Close()

	projects := make([]models.Project, 0)
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.APIKey, &p.CreatedAt); err != nil {
			log.Printf("failed to scan project row: %v", err)
			continue
		}
		projects = append(projects, p)
	}

	writeJSON(w, http.StatusOK, projects)
}

// Get godoc
//
//	@Summary      Get project
//	@Description  Returns a single project by ID.
//	@Tags         projects
//	@Produce      json
//	@Security     BearerAuth
//	@Param        projectId path     string true "Project UUID"
//	@Success      200       {object} models.Project
//	@Failure      401       {object} ErrorResponse
//	@Failure      404       {object} ErrorResponse
//	@Router       /api/projects/{projectId} [get]
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	var project models.Project
	query := `SELECT id, name, api_key, created_at FROM projects WHERE id = $1`
	err := h.db.QueryRow(r.Context(), query, projectID).
		Scan(&project.ID, &project.Name, &project.APIKey, &project.CreatedAt)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	writeJSON(w, http.StatusOK, project)
}

// Delete godoc
//
//	@Summary      Delete project
//	@Description  Permanently deletes a project and all its issues and events (cascade).
//	@Tags         projects
//	@Security     BearerAuth
//	@Param        projectId path string true "Project UUID"
//	@Success      204
//	@Failure      401 {object} ErrorResponse
//	@Failure      404 {object} ErrorResponse
//	@Router       /api/projects/{projectId} [delete]
func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	result, err := h.db.Exec(r.Context(), `DELETE FROM projects WHERE id = $1`, projectID)
	if err != nil {
		log.Printf("failed to delete project [id=%s]: %v", projectID, err)
		writeError(w, http.StatusInternalServerError, "failed to delete project")
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func generateAPIKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
