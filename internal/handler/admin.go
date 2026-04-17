package handler

import (
	"encoding/json"
	"net/http"

	"github.com/Ryohskay/portacconti/internal/domain"
	"github.com/Ryohskay/portacconti/internal/repository"
	"github.com/Ryohskay/portacconti/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AdminHandler struct {
	auth  *service.AuthService
	users repository.UserRepository
}

func NewAdminHandler(auth *service.AuthService, users repository.UserRepository) *AdminHandler {
	return &AdminHandler{auth: auth, users: users}
}

// POST /api/v1/admin/users  (manager: create staff)
func (h *AdminHandler) CreateStaff(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Role     string `json:"role"`
		Locale   string `json:"locale"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Role != string(domain.RoleCounsellor) && req.Role != string(domain.RoleManager) {
		respondError(w, http.StatusBadRequest, "role must be 'counsellor' or 'manager'")
		return
	}

	user, err := h.auth.CreateStaffUser(r.Context(), service.RegisterInput{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
		Role:     domain.Role(req.Role),
		Locale:   req.Locale,
	})
	if err != nil {
		if err == service.ErrEmailTaken {
			respondError(w, http.StatusConflict, "email already in use")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to create user")
		return
	}
	respondCreated(w, user)
}

// GET /api/v1/admin/users  (manager)
func (h *AdminHandler) ListStaff(w http.ResponseWriter, r *http.Request) {
	counsellors, err := h.users.ListByRole(r.Context(), domain.RoleCounsellor)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list counsellors")
		return
	}
	managers, err := h.users.ListByRole(r.Context(), domain.RoleManager)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list managers")
		return
	}
	respondOK(w, map[string]any{
		"counsellors": counsellors,
		"managers":    managers,
	})
}

// PATCH /api/v1/admin/users/{id}  (manager: activate/deactivate)
func (h *AdminHandler) UpdateStaff(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req struct {
		IsActive *bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.users.GetByID(r.Context(), id)
	if err != nil || user == nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if err := h.users.Update(r.Context(), user); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	respondOK(w, user)
}
