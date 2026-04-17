package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Ryohskay/portacconti/internal/domain"
	"github.com/Ryohskay/portacconti/internal/repository"
	pgrepo "github.com/Ryohskay/portacconti/internal/repository/postgres"
	"github.com/go-chi/chi/v5"
)

type QuestionnaireHandler struct {
	questionnaire repository.QuestionnaireRepository
}

func NewQuestionnaireHandler(questionnaire repository.QuestionnaireRepository) *QuestionnaireHandler {
	return &QuestionnaireHandler{questionnaire: questionnaire}
}

// GET /api/v1/questionnaire/{token}  (no auth — token is the credential)
func (h *QuestionnaireHandler) GetForm(w http.ResponseWriter, r *http.Request) {
	rawToken := chi.URLParam(r, "token")
	hash := pgrepo.HashToken(rawToken)

	tok, err := h.questionnaire.GetToken(r.Context(), hash)
	if err != nil || tok == nil {
		respondError(w, http.StatusNotFound, "questionnaire not found")
		return
	}
	if tok.UsedAt != nil {
		respondError(w, http.StatusGone, "questionnaire already submitted")
		return
	}
	if time.Now().After(tok.ExpiresAt) {
		respondError(w, http.StatusGone, "questionnaire link has expired")
		return
	}

	tmpl, err := h.questionnaire.GetActiveTemplate(r.Context(), r.URL.Query().Get("locale"))
	if err != nil || tmpl == nil {
		respondError(w, http.StatusNotFound, "questionnaire template not found")
		return
	}
	respondOK(w, map[string]any{
		"template":       tmpl,
		"appointment_id": tok.AppointmentID,
	})
}

// POST /api/v1/questionnaire/{token}  (no auth)
func (h *QuestionnaireHandler) Submit(w http.ResponseWriter, r *http.Request) {
	rawToken := chi.URLParam(r, "token")
	hash := pgrepo.HashToken(rawToken)

	tok, err := h.questionnaire.GetToken(r.Context(), hash)
	if err != nil || tok == nil {
		respondError(w, http.StatusNotFound, "questionnaire not found")
		return
	}
	if tok.UsedAt != nil {
		respondError(w, http.StatusGone, "questionnaire already submitted")
		return
	}
	if time.Now().After(tok.ExpiresAt) {
		respondError(w, http.StatusGone, "questionnaire link has expired")
		return
	}

	var req struct {
		Answers map[string]string `json:"answers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Answers) == 0 {
		respondError(w, http.StatusBadRequest, "answers are required")
		return
	}

	resp := &domain.QuestionnaireResponse{
		AppointmentID: tok.AppointmentID,
		TemplateID:    tok.TemplateID,
		Answers:       req.Answers,
	}
	if err := h.questionnaire.SaveResponse(r.Context(), resp); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save response")
		return
	}

	if err := h.questionnaire.MarkTokenUsed(r.Context(), hash); err != nil {
		// Non-fatal — log in production
		_ = err
	}

	respondCreated(w, map[string]string{"message": "questionnaire submitted successfully"})
}
