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

type ShiftHandler struct {
	shifts repository.ShiftRepository
}

func NewShiftHandler(shifts repository.ShiftRepository) *ShiftHandler {
	return &ShiftHandler{shifts: shifts}
}

// POST /api/v1/shifts  (manager)
func (h *ShiftHandler) CreateShift(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StartsAt time.Time `json:"starts_at"`
		EndsAt   time.Time `json:"ends_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	managerID, err := uuid.Parse(middleware.UserIDFromContext(r.Context()))
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid user")
		return
	}
	shift := &domain.Shift{
		ManagerID: managerID,
		StartsAt:  req.StartsAt,
		EndsAt:    req.EndsAt,
	}
	if err := h.shifts.CreateShift(r.Context(), shift); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create shift")
		return
	}
	respondCreated(w, shift)
}

// GET /api/v1/shifts  (manager)
func (h *ShiftHandler) ListShifts(w http.ResponseWriter, r *http.Request) {
	from := parseTimeQuery(r, "from", time.Now())
	to := parseTimeQuery(r, "to", time.Now().AddDate(0, 1, 0))
	shifts, err := h.shifts.ListShifts(r.Context(), from, to)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list shifts")
		return
	}
	respondOK(w, shifts)
}

// DELETE /api/v1/shifts/{id}  (manager)
func (h *ShiftHandler) CloseShift(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid shift id")
		return
	}
	if err := h.shifts.CloseShift(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to close shift")
		return
	}
	respondOK(w, map[string]string{"message": "shift closed"})
}

// POST /api/v1/shifts/{id}/timeslots  (manager)
func (h *ShiftHandler) GenerateTimeslots(w http.ResponseWriter, r *http.Request) {
	shiftID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid shift id")
		return
	}
	var req struct {
		SlotDurationMinutes int `json:"slot_duration_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SlotDurationMinutes <= 0 {
		req.SlotDurationMinutes = 50
	}

	shift, err := h.shifts.GetShiftByID(r.Context(), shiftID)
	if err != nil || shift == nil {
		respondError(w, http.StatusNotFound, "shift not found")
		return
	}

	duration := time.Duration(req.SlotDurationMinutes) * time.Minute
	var slots []*domain.Timeslot
	for t := shift.StartsAt; t.Add(duration).Before(shift.EndsAt) || t.Add(duration).Equal(shift.EndsAt); t = t.Add(duration) {
		slot := &domain.Timeslot{
			ShiftID:  shiftID,
			StartsAt: t,
			EndsAt:   t.Add(duration),
		}
		if err := h.shifts.CreateTimeslot(r.Context(), slot); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to create timeslot")
			return
		}
		slots = append(slots, slot)
	}
	respondCreated(w, slots)
}

// GET /api/v1/shifts/{id}/timeslots  (manager)
func (h *ShiftHandler) ListTimeslotsByShift(w http.ResponseWriter, r *http.Request) {
	shiftID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid shift id")
		return
	}
	slots, err := h.shifts.ListTimeslotsByShift(r.Context(), shiftID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list timeslots")
		return
	}
	respondOK(w, slots)
}

// POST /api/v1/shifts/{id}/counsellors  (manager)
func (h *ShiftHandler) AddCounsellor(w http.ResponseWriter, r *http.Request) {
	shiftID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid shift id")
		return
	}
	var req struct {
		CounsellorID string `json:"counsellor_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	counsellorID, err := uuid.Parse(req.CounsellorID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid counsellor_id")
		return
	}
	if err := h.shifts.AddCounsellorToShift(r.Context(), shiftID, counsellorID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to add counsellor")
		return
	}
	respondOK(w, map[string]string{"message": "counsellor added to shift"})
}

// DELETE /api/v1/shifts/{id}/counsellors/{counsellor_id}  (manager)
func (h *ShiftHandler) RemoveCounsellor(w http.ResponseWriter, r *http.Request) {
	shiftID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid shift id")
		return
	}
	counsellorID, err := uuid.Parse(chi.URLParam(r, "counsellor_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid counsellor_id")
		return
	}
	if err := h.shifts.RemoveCounsellorFromShift(r.Context(), shiftID, counsellorID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to remove counsellor")
		return
	}
	respondOK(w, map[string]string{"message": "counsellor removed from shift"})
}

// GET /api/v1/timeslots/available  (patient)
func (h *ShiftHandler) ListAvailableTimeslots(w http.ResponseWriter, r *http.Request) {
	from := parseTimeQuery(r, "from", time.Now())
	to := parseTimeQuery(r, "to", time.Now().AddDate(0, 1, 0))
	slots, err := h.shifts.ListAvailableTimeslots(r.Context(), from, to)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list timeslots")
		return
	}
	respondOK(w, slots)
}

func parseTimeQuery(r *http.Request, key string, fallback time.Time) time.Time {
	s := r.URL.Query().Get(key)
	if s == "" {
		return fallback
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fallback
	}
	return t
}
