package domain

import (
	"time"

	"github.com/google/uuid"
)

type PaymentStatus string

const (
	PaymentCreated    PaymentStatus = "created"
	PaymentProcessing PaymentStatus = "processing"
	PaymentSucceeded  PaymentStatus = "succeeded"
	PaymentFailed     PaymentStatus = "failed"
	PaymentRefunded   PaymentStatus = "refunded"
)

type Payment struct {
	ID                   uuid.UUID     `json:"id"`
	AppointmentID        uuid.UUID     `json:"appointment_id"`
	PatientID            uuid.UUID     `json:"patient_id"`
	StripePaymentIntent  string        `json:"stripe_payment_intent"`
	AmountJPY            int64         `json:"amount_jpy"`
	Currency             string        `json:"currency"`
	Status               PaymentStatus `json:"status"`
	StripeEventID        string        `json:"stripe_event_id,omitempty"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
}
