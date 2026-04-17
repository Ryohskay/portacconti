package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Ryohskay/portacconti/internal/domain"
	"github.com/Ryohskay/portacconti/internal/middleware"
	"github.com/Ryohskay/portacconti/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Locale   string `json:"locale"`
}

// POST /api/v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" || req.Name == "" {
		respondError(w, http.StatusBadRequest, "email, password, and name are required")
		return
	}
	if len(req.Password) < 8 {
		respondError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	user, tokens, err := h.auth.Register(r.Context(), service.RegisterInput{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
		Locale:   req.Locale,
		Role:     domain.RolePatient,
	})
	if err != nil {
		if errors.Is(err, service.ErrEmailTaken) {
			respondError(w, http.StatusConflict, "email already in use")
			return
		}
		respondError(w, http.StatusInternalServerError, "registration failed")
		return
	}

	setRefreshCookie(w, tokens.RefreshToken)
	respondCreated(w, map[string]any{
		"user":         user,
		"access_token": tokens.AccessToken,
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, tokens, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			respondError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		respondError(w, http.StatusInternalServerError, "login failed")
		return
	}

	setRefreshCookie(w, tokens.RefreshToken)
	respondOK(w, map[string]any{
		"user":         user,
		"access_token": tokens.AccessToken,
	})
}

// POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		respondError(w, http.StatusUnauthorized, "missing refresh token")
		return
	}

	user, tokens, err := h.auth.Refresh(r.Context(), cookie.Value)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	setRefreshCookie(w, tokens.RefreshToken)
	respondOK(w, map[string]any{
		"user":         user,
		"access_token": tokens.AccessToken,
	})
}

// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		_ = h.auth.Logout(r.Context(), cookie.Value)
	}
	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1/auth/refresh",
	})
	respondOK(w, map[string]string{"message": "logged out"})
}

// GET /api/v1/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	respondOK(w, map[string]string{"user_id": userID, "role": middleware.RoleFromContext(r.Context())})
}

func setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1/auth/refresh",
	})
}
