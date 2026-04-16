package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Aspors/errhub-backend/internal/service/sourcemap"
	"github.com/Aspors/errhub-backend/internal/storage/s3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxUploadSize = 32 << 20 // 32 MB

type SourcemapHandler struct {
	db      *pgxpool.Pool
	storage *s3.Storage
	srcSvc  *sourcemap.Service
}

func NewSourcemapHandler(db *pgxpool.Pool, storage *s3.Storage, srcSvc *sourcemap.Service) *SourcemapHandler {
	return &SourcemapHandler{db: db, storage: storage, srcSvc: srcSvc}
}

// UploadSourcemapResponse is returned after a successful source map upload.
type UploadSourcemapResponse struct {
	ObjectKey string `json:"object_key" example:"proj-uuid/a3f9c2b/index-abc123.js.map"`
	Release   string `json:"release"    example:"a3f9c2b"`
	Filename  string `json:"filename"   example:"index-abc123.js.map"`
}

// Upload godoc
//
//	@Summary      Upload source map
//	@Description  Uploads a `.map` file for a specific build release. Called automatically from CI after `npm run build`. The release string is typically the short git commit hash (`$GITHUB_SHA` or `git rev-parse --short HEAD`). Files are stored in MinIO and tracked in the DB for automatic retention management (deleted after 3 days of no use).
//	@Tags         sourcemaps
//	@Accept       multipart/form-data
//	@Produce      json
//	@Security     BearerAuth
//	@Param        projectId path     string true "Project UUID"
//	@Param        release   path     string true "Build release string (e.g. git commit hash)"
//	@Param        file      formData file   true "Source map file (.map)"
//	@Success      201       {object} UploadSourcemapResponse
//	@Failure      400       {object} ErrorResponse
//	@Failure      401       {object} ErrorResponse
//	@Failure      422       {object} ErrorResponse "only .map files accepted"
//	@Failure      500       {object} ErrorResponse
//	@Router       /api/projects/{projectId}/releases/{release}/sourcemaps [post]
func (h *SourcemapHandler) Upload(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	release := r.PathValue("release")

	// Authenticate via project API key: Authorization: Bearer <api_key>
	apiKey := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if apiKey == "" {
		writeError(w, http.StatusUnauthorized, "missing API key")
		return
	}
	var dummy string
	err := h.db.QueryRow(r.Context(),
		`SELECT id FROM projects WHERE id = $1 AND api_key = $2`,
		projectID, apiKey).Scan(&dummy)
	if err == pgx.ErrNoRows || dummy == "" {
		writeError(w, http.StatusUnauthorized, "invalid API key")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify API key")
		return
	}

	if h.storage == nil {
		writeError(w, http.StatusServiceUnavailable, "source map storage is not configured")
		return
	}

	if release == "" {
		writeError(w, http.StatusBadRequest, "release is required")
		return
	}
	if strings.ContainsAny(release, "/\\..") {
		writeError(w, http.StatusBadRequest, "release contains invalid characters")
		return
	}

	// Limit total request body size before touching multipart.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	// Stream multipart directly — avoids buffering the file to disk/memory.
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	var (
		filename  string
		objectKey string
		uploaded  bool
	)

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read multipart form")
			return
		}

		if part.FormName() != "file" {
			part.Close()
			continue
		}

		filename = filepath.Base(part.FileName())
		if !strings.HasSuffix(filename, ".map") {
			part.Close()
			writeError(w, http.StatusUnprocessableEntity, "only .map files are accepted")
			return
		}

		objectKey = fmt.Sprintf("%s/%s/%s", projectID, release, filename)

		// size=-1: MinIO uses chunked upload, no full-file buffering required.
		if err := h.storage.Upload(r.Context(), objectKey, part, -1); err != nil {
			part.Close()
			log.Printf("failed to upload source map [key=%s]: %v", objectKey, err)
			writeError(w, http.StatusInternalServerError, "failed to store source map")
			return
		}
		part.Close()
		uploaded = true
		break
	}

	if !uploaded {
		writeError(w, http.StatusBadRequest, "missing 'file' field in form")
		return
	}

	_, err = h.db.Exec(r.Context(), `
		INSERT INTO sourcemap_files (project_id, release, object_key, size_bytes)
		VALUES ($1, $2, $3, 0)
		ON CONFLICT (project_id, object_key) DO UPDATE SET
			last_used_at = NOW()`,
		projectID, release, objectKey)
	if err != nil {
		log.Printf("failed to record source map in DB [key=%s]: %v", objectKey, err)
		// Roll back the MinIO upload so we don't leave an orphaned file.
		if delErr := h.storage.Delete(r.Context(), objectKey); delErr != nil {
			log.Printf("WARNING: orphaned file in MinIO [key=%s]: %v", objectKey, delErr)
		}
		writeError(w, http.StatusInternalServerError, "failed to record source map")
		return
	}

	// Retroactively deobfuscate events that arrived before this source map.
	// Wrapped in panic recovery so a bad source map can't crash the server.
	if h.srcSvc != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("sourcemap: panic in ResolveEventsForRelease [project=%s release=%s]: %v",
						projectID, release, r)
				}
			}()
			h.srcSvc.ResolveEventsForRelease(projectID, release)
		}()
	}

	writeJSON(w, http.StatusCreated, UploadSourcemapResponse{
		ObjectKey: objectKey,
		Release:   release,
		Filename:  filename,
	})
}
