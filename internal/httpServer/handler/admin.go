package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/Aspors/errhub-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type AdminHandler struct {
	db       *pgxpool.Pool
	adminKey string
}

func NewAdminHandler(db *pgxpool.Pool, adminKey string) *AdminHandler {
	return &AdminHandler{db: db, adminKey: adminKey}
}

// CreateUserRequest is the payload for POST /api/admin/users.
type CreateUserRequest struct {
	Email    string `json:"email"    example:"dev@company.com"`
	Password string `json:"password" example:"supersecret123"`
}

func (h *AdminHandler) adminKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Admin-Key")
		if key == "" {
			key = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}
		if key != h.adminKey || h.adminKey == "" {
			writeError(w, http.StatusForbidden, "invalid or missing admin key")
			return
		}
		next(w, r)
	}
}

// CreateUser godoc
//
//	@Summary     Create user
//	@Description Manually provision a new user account. Access is granted by the admin, not via self-registration. Requires X-Admin-Key header.
//	@Tags        admin
//	@Accept      json
//	@Produce     json
//	@Param       X-Admin-Key header   string            true "Admin key from ADMIN_KEY env var"
//	@Param       body        body     CreateUserRequest true "New user credentials"
//	@Success     201         {object} models.User
//	@Failure     400         {object} ErrorResponse
//	@Failure     403         {object} ErrorResponse "invalid or missing admin key"
//	@Failure     409         {object} ErrorResponse "email already registered"
//	@Failure     422         {object} ErrorResponse
//	@Router      /api/admin/users [post]
func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	h.adminKeyMiddleware(func(w http.ResponseWriter, r *http.Request) {
		var req CreateUserRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if req.Email == "" || req.Password == "" {
			writeError(w, http.StatusUnprocessableEntity, "email and password are required")
			return
		}
		if len(req.Password) < 8 {
			writeError(w, http.StatusUnprocessableEntity, "password must be at least 8 characters")
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("bcrypt error: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to create user")
			return
		}

		var user models.User
		query := `
			INSERT INTO users (email, password_hash)
			VALUES ($1, $2)
			RETURNING id, email, created_at`
		err = h.db.QueryRow(r.Context(), query, req.Email, string(hash)).
			Scan(&user.ID, &user.Email, &user.CreatedAt)
		if err != nil {
			if isUniqueViolation(err) {
				writeError(w, http.StatusConflict, "email already registered")
				return
			}
			log.Printf("failed to create user: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to create user")
			return
		}

		writeJSON(w, http.StatusCreated, user)
	})(w, r)
}
