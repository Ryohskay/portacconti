package domain

import (
	"time"

	"github.com/google/uuid"
)

type AppointmentStatus string

const (
	StatusPendingPayment AppointmentStatus = "pending_payment"
	StatusConfirmed      AppointmentStatus = "confirmed"
	StatusInProgress     AppointmentStatus = "in_progress"
	StatusCompleted      AppointmentStatus = "completed"
	StatusCancelled      AppointmentStatus = "cancelled"
	StatusNoShow         AppointmentStatus = "no_show"
)

type Appointment struct {
	ID                 uuid.UUID         `json:"id"`
	TimeslotID         uuid.UUID         `json:"timeslot_id"`
	PatientID          uuid.UUID         `json:"patient_id"`
	CounsellorID       *uuid.UUID        `json:"counsellor_id,omitempty"`
	Status             AppointmentStatus `json:"status"`
	MeetingURL         string            `json:"meeting_url,omitempty"`
	CancellationReason string            `json:"cancellation_reason,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`

	// Joined fields
	Timeslot   *Timeslot `json:"timeslot,omitempty"`
	Patient    *User     `json:"patient,omitempty"`
	Counsellor *User     `json:"counsellor,omitempty"`
}

type PatientRecord struct {
	ID            uuid.UUID `json:"id"`
	AppointmentID uuid.UUID `json:"appointment_id"`
	AuthorID      uuid.UUID `json:"author_id"`
	Content       string    `json:"content"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	Author *User `json:"author,omitempty"`
}
