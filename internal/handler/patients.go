package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Ryohskay/portacconti/internal/domain"
	"github.com/Ryohskay/portacconti/internal/middleware"
	"github.com/Ryohskay/portacconti/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type PatientHandler struct {
	users repository.UserRepository
}

func NewPatientHandler(users repository.UserRepository) *PatientHandler {
	return &PatientHandler{users: users}
}

// GET /api/v1/patients  (manager)
func (h *PatientHandler) ListPatients(w http.ResponseWriter, r *http.Request) {
	patients, err := h.users.ListByRole(r.Context(), domain.RolePatient)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list patients")
		return
	}
	respondOK(w, patients)
}

// GET /api/v1/patients/{id}  (manager, counsellor with assignment, patient self)
func (h *PatientHandler) GetPatient(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid patient id")
		return
	}

	// Patients can only view their own profile
	role := middleware.RoleFromContext(r.Context())
	callerID := middleware.UserIDFromContext(r.Context())
	if domain.Role(role) == domain.RolePatient && callerID != id.String() {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	patient, err := h.users.GetByID(r.Context(), id)
	if err != nil || patient == nil {
		respondError(w, http.StatusNotFound, "patient not found")
		return
	}
	if patient.Role != domain.RolePatient {
		respondError(w, http.StatusNotFound, "patient not found")
		return
	}
	respondOK(w, patient)
}

// PUT /api/v1/patients/{id}  (manager, patient self)
func (h *PatientHandler) UpdatePatient(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid patient id")
		return
	}

	role := middleware.RoleFromContext(r.Context())
	callerID := middleware.UserIDFromContext(r.Context())
	if domain.Role(role) == domain.RolePatient && callerID != id.String() {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	patient, err := h.users.GetByID(r.Context(), id)
	if err != nil || patient == nil {
		respondError(w, http.StatusNotFound, "patient not found")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Phone       string `json:"phone"`
		DateOfBirth string `json:"date_of_birth"` // YYYY-MM-DD
		Locale      string `json:"locale"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != "" {
		patient.Name = req.Name
	}
	if req.Phone != "" {
		patient.Phone = req.Phone
	}
	if req.Locale != "" {
		patient.Locale = req.Locale
	}
	if req.DateOfBirth != "" {
		dob, err := time.Parse(time.DateOnly, req.DateOfBirth)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid date_of_birth format, use YYYY-MM-DD")
			return
		}
		patient.DateOfBirth = &dob
	}

	if err := h.users.Update(r.Context(), patient); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update patient")
		return
	}
	respondOK(w, patient)
}

// GET /api/v1/counsellors  (manager)
func (h *PatientHandler) ListCounsellors(w http.ResponseWriter, r *http.Request) {
	counsellors, err := h.users.ListByRole(r.Context(), domain.RoleCounsellor)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list counsellors")
		return
	}
	respondOK(w, counsellors)
}
