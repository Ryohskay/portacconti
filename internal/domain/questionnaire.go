package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type QuestionType string

const (
	QuestionText     QuestionType = "text"
	QuestionSelect   QuestionType = "select"
	QuestionMultiple QuestionType = "multiple"
	QuestionBoolean  QuestionType = "boolean"
)

type Question struct {
	ID       string            `json:"id"`
	Type     QuestionType      `json:"type"`
	TextEN   string            `json:"text_en"`
	TextJA   string            `json:"text_ja"`
	Options  []QuestionOption  `json:"options,omitempty"`
	Required bool              `json:"required"`
}

type QuestionOption struct {
	Value  string `json:"value"`
	LabelEN string `json:"label_en"`
	LabelJA string `json:"label_ja"`
}

type QuestionnaireTemplate struct {
	ID        uuid.UUID  `json:"id"`
	Locale    string     `json:"locale"`
	Title     string     `json:"title"`
	Questions []Question `json:"questions"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
}

func (t *QuestionnaireTemplate) QuestionsJSON() ([]byte, error) {
	return json.Marshal(t.Questions)
}

type QuestionnaireToken struct {
	ID            uuid.UUID  `json:"id"`
	AppointmentID uuid.UUID  `json:"appointment_id"`
	TemplateID    uuid.UUID  `json:"template_id"`
	TokenHash     string     `json:"-"`
	ExpiresAt     time.Time  `json:"expires_at"`
	UsedAt        *time.Time `json:"used_at,omitempty"`
}

type QuestionnaireResponse struct {
	ID            uuid.UUID         `json:"id"`
	AppointmentID uuid.UUID         `json:"appointment_id"`
	TemplateID    uuid.UUID         `json:"template_id"`
	Answers       map[string]string `json:"answers"`
	SubmittedAt   time.Time         `json:"submitted_at"`
}
