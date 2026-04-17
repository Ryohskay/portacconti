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

type MessageHandler struct {
	messages     repository.MessageRepository
	appointments repository.AppointmentRepository
	users        repository.UserRepository
	notify       *service.NotificationService
}

func NewMessageHandler(
	messages repository.MessageRepository,
	appointments repository.AppointmentRepository,
	users repository.UserRepository,
	notify *service.NotificationService,
) *MessageHandler {
	return &MessageHandler{messages: messages, appointments: appointments, users: users, notify: notify}
}

// POST /api/v1/appointments/{id}/messages  (counsellor or manager)
func (h *MessageHandler) Send(w http.ResponseWriter, r *http.Request) {
	apptID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid appointment id")
		return
	}
	senderID, _ := uuid.Parse(middleware.UserIDFromContext(r.Context()))

	appt, err := h.appointments.GetByID(r.Context(), apptID)
	if err != nil || appt == nil {
		respondError(w, http.StatusNotFound, "appointment not found")
		return
	}

	var req struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Body == "" {
		respondError(w, http.StatusBadRequest, "body is required")
		return
	}

	msg := &domain.Message{
		AppointmentID: apptID,
		SenderID:      senderID,
		RecipientID:   appt.PatientID,
		Subject:       req.Subject,
		Body:          req.Body,
	}
	if err := h.messages.Create(r.Context(), msg); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to send message")
		return
	}

	// Also send via email
	patient, err := h.users.GetByID(r.Context(), appt.PatientID)
	sender, err2 := h.users.GetByID(r.Context(), senderID)
	if err == nil && err2 == nil && patient != nil && sender != nil {
		go h.notify.SendMessageEmail(r.Context(), patient.Email, sender.Name, req.Subject, req.Body)
	}

	respondCreated(w, msg)
}

// GET /api/v1/appointments/{id}/messages  (counsellor, manager, patient own)
func (h *MessageHandler) List(w http.ResponseWriter, r *http.Request) {
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

	role := middleware.RoleFromContext(r.Context())
	userIDStr := middleware.UserIDFromContext(r.Context())
	userID, _ := uuid.Parse(userIDStr)
	if domain.Role(role) == domain.RolePatient && appt.PatientID != userID {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	msgs, err := h.messages.ListByAppointment(r.Context(), apptID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list messages")
		return
	}
	respondOK(w, msgs)
}
