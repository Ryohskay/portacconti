package handler

import (
	"encoding/json"
	"net/http"

	"github.com/Ryohskay/portacconti/internal/domain"
	"github.com/Ryohskay/portacconti/internal/middleware"
	"github.com/Ryohskay/portacconti/internal/repository"
	"github.com/Ryohskay/portacconti/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AppointmentHandler struct {
	appointments repository.AppointmentRepository
	users        repository.UserRepository
	booking      *service.BookingService
}

func NewAppointmentHandler(
	appointments repository.AppointmentRepository,
	users repository.UserRepository,
	booking *service.BookingService,
) *AppointmentHandler {
	return &AppointmentHandler{appointments: appointments, users: users, booking: booking}
}

// POST /api/v1/appointments  (patient)
func (h *AppointmentHandler) InitiateBooking(w http.ResponseWriter, r *http.Request) {
	patientID, err := uuid.Parse(middleware.UserIDFromContext(r.Context()))
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid user")
		return
	}
	var req struct {
		TimeslotID string `json:"timeslot_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	slotID, err := uuid.Parse(req.TimeslotID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid timeslot_id")
		return
	}

	result, err := h.booking.InitiateBooking(r.Context(), patientID, slotID)
	if err != nil {
		switch err {
		case service.ErrSlotNotFound:
			respondError(w, http.StatusNotFound, "timeslot not found")
		case service.ErrSlotUnavailable:
			respondError(w, http.StatusConflict, "timeslot is no longer available")
		default:
			respondError(w, http.StatusInternalServerError, "failed to initiate booking")
		}
		return
	}

	respondCreated(w, map[string]any{
		"appointment":           result.Appointment,
		"payment_client_secret": result.PaymentClientSecret,
	})
}

// GET /api/v1/appointments  (all roles)
func (h *AppointmentHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, _ := uuid.Parse(middleware.UserIDFromContext(ctx))
	role := middleware.RoleFromContext(ctx)

	var appts []*domain.Appointment
	var err error
	switch domain.Role(role) {
	case domain.RoleManager:
		appts, err = h.appointments.ListAll(ctx)
	case domain.RoleCounsellor:
		appts, err = h.appointments.ListByCounsellor(ctx, userID)
	default:
		appts, err = h.appointments.ListByPatient(ctx, userID)
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list appointments")
		return
	}
	respondOK(w, appts)
}

// GET /api/v1/appointments/{id}
func (h *AppointmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	apptID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid appointment id")
		return
	}
	appt, err := h.appointments.GetByID(r.Context(), apptID)
	if err != nil || appt == nil {
		respondError(w, http.StatusNotFound, "appointment not found")
		return
	}
	if !canAccessAppointment(r, appt) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	respondOK(w, appt)
}

// PATCH /api/v1/appointments/{id}  (manager)
func (h *AppointmentHandler) UpdateByManager(w http.ResponseWriter, r *http.Request) {
	apptID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid appointment id")
		return
	}
	var req struct {
		NewTimeslotID  string `json:"new_timeslot_id,omitempty"`
		CounsellorID   string `json:"counsellor_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var appt *domain.Appointment
	if req.NewTimeslotID != "" {
		slotID, err := uuid.Parse(req.NewTimeslotID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid new_timeslot_id")
			return
		}
		appt, err = h.booking.RescheduleAppointment(r.Context(), apptID, slotID)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if req.CounsellorID != "" {
		cID, err := uuid.Parse(req.CounsellorID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid counsellor_id")
			return
		}
		appt, err = h.booking.AssignCounsellor(r.Context(), apptID, cID)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	respondOK(w, appt)
}

// PATCH /api/v1/appointments/{id}/status  (counsellor or manager)
func (h *AppointmentHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	apptID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid appointment id")
		return
	}
	var req struct {
		Status     string `json:"status"`
		MeetingURL string `json:"meeting_url,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	appt, err := h.appointments.GetByID(r.Context(), apptID)
	if err != nil || appt == nil {
		respondError(w, http.StatusNotFound, "appointment not found")
		return
	}
	if !canAccessAppointment(r, appt) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	if req.Status == string(domain.StatusInProgress) && req.MeetingURL != "" {
		appt, err = h.booking.StartMeeting(r.Context(), apptID, req.MeetingURL)
	} else {
		appt.Status = domain.AppointmentStatus(req.Status)
		err = h.appointments.Update(r.Context(), appt)
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update status")
		return
	}
	respondOK(w, appt)
}

// DELETE /api/v1/appointments/{id}  (manager)
func (h *AppointmentHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	apptID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid appointment id")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	appt, err := h.booking.CancelAppointment(r.Context(), apptID, req.Reason)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondOK(w, appt)
}

// POST /api/v1/appointments/{id}/records  (counsellor or manager)
func (h *AppointmentHandler) AddRecord(w http.ResponseWriter, r *http.Request) {
	apptID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid appointment id")
		return
	}
	authorID, _ := uuid.Parse(middleware.UserIDFromContext(r.Context()))

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
		respondError(w, http.StatusBadRequest, "content is required")
		return
	}

	appt, err := h.appointments.GetByID(r.Context(), apptID)
	if err != nil || appt == nil {
		respondError(w, http.StatusNotFound, "appointment not found")
		return
	}
	if !canAccessAppointment(r, appt) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	rec := &domain.PatientRecord{
		AppointmentID: apptID,
		AuthorID:      authorID,
		Content:       req.Content,
	}
	if err := h.appointments.AddRecord(r.Context(), rec); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to add record")
		return
	}
	respondCreated(w, rec)
}

// GET /api/v1/appointments/{id}/records  (counsellor or manager)
func (h *AppointmentHandler) ListRecords(w http.ResponseWriter, r *http.Request) {
	apptID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid appointment id")
		return
	}
	appt, err := h.appointments.GetByID(r.Context(), apptID)
	if err != nil || appt == nil {
		respondError(w, http.StatusNotFound, "appointment not found")
		return
	}
	if !canAccessAppointment(r, appt) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	recs, err := h.appointments.ListRecords(r.Context(), apptID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list records")
		return
	}
	respondOK(w, recs)
}

// PUT /api/v1/appointments/{id}/records/{record_id}  (counsellor or manager)
func (h *AppointmentHandler) UpdateRecord(w http.ResponseWriter, r *http.Request) {
	recID, err := uuid.Parse(chi.URLParam(r, "record_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid record id")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
		respondError(w, http.StatusBadRequest, "content is required")
		return
	}

	rec, err := h.appointments.GetRecord(r.Context(), recID)
	if err != nil || rec == nil {
		respondError(w, http.StatusNotFound, "record not found")
		return
	}
	rec.Content = req.Content
	if err := h.appointments.UpdateRecord(r.Context(), rec); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update record")
		return
	}
	respondOK(w, rec)
}

func canAccessAppointment(r *http.Request, appt *domain.Appointment) bool {
	role := middleware.RoleFromContext(r.Context())
	userIDStr := middleware.UserIDFromContext(r.Context())
	userID, _ := uuid.Parse(userIDStr)

	switch domain.Role(role) {
	case domain.RoleManager:
		return true
	case domain.RoleCounsellor:
		return appt.CounsellorID != nil && *appt.CounsellorID == userID
	default: // patient
		return appt.PatientID == userID
	}
}
