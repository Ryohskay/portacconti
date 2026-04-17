package domain

import (
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID            uuid.UUID  `json:"id"`
	AppointmentID uuid.UUID  `json:"appointment_id"`
	SenderID      uuid.UUID  `json:"sender_id"`
	RecipientID   uuid.UUID  `json:"recipient_id"`
	Subject       string     `json:"subject,omitempty"`
	Body          string     `json:"body"`
	SentAt        time.Time  `json:"sent_at"`
	ReadAt        *time.Time `json:"read_at,omitempty"`

	Sender    *User `json:"sender,omitempty"`
	Recipient *User `json:"recipient,omitempty"`
}
