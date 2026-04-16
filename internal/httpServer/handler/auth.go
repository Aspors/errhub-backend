package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Aspors/errhub-backend/internal/httpserver/middleware"
	"github.com/Aspors/errhub-backend/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db        *pgxpool.Pool
	jwtSecret string
}

func NewAuthHandler(db *pgxpool.Pool, jwtSecret string) *AuthHandler {
	return &AuthHandler{db: db, jwtSecret: jwtSecret}
}

// LoginRequest is the payload for POST /api/auth/login.
type LoginRequest struct {
	Email    string `json:"email"    example:"dev@company.com"`
	Password string `json:"password" example:"supersecret"`
}

// AuthResponse is returned on successful login.
type AuthResponse struct {
	User models.User `json:"user"`
}

// Login godoc
//
//	@Summary     Login
//	@Description Authenticate with email and password. Returns a JWT valid for 24 hours.
//	@Tags        auth
//	@Accept      json
//	@Produce     json
//	@Param       body body     LoginRequest true "Credentials"
//	@Success     200  {object} AuthResponse
//	@Failure     400  {object} ErrorResponse
//	@Failure     401  {object} ErrorResponse "invalid email or password"
//	@Router      /api/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var user models.User
	var passwordHash string
	query := `SELECT id, email, password_hash, created_at FROM users WHERE email = $1`
	err := h.db.QueryRow(r.Context(), query, req.Email).
		Scan(&user.ID, &user.Email, &passwordHash, &user.CreatedAt)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := h.generateToken(user.ID)
	if err != nil {
		log.Printf("failed to generate token: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		MaxAge:   24 * 60 * 60,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, AuthResponse{User: user})
}

// Logout godoc
//
//	@Summary     Logout
//	@Description Clears the auth cookie.
//	@Tags        auth
//	@Success     204
//	@Router      /api/auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) generateToken(userID string) (string, error) {
	claims := middleware.Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}

// isUniqueViolation detects PostgreSQL unique constraint violation (error code 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
