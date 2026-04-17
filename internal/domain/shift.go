package domain

import (
	"time"

	"github.com/google/uuid"
)

type ShiftStatus string

const (
	ShiftOpen   ShiftStatus = "open"
	ShiftClosed ShiftStatus = "closed"
)

type Shift struct {
	ID          uuid.UUID   `json:"id"`
	ManagerID   uuid.UUID   `json:"manager_id"`
	StartsAt    time.Time   `json:"starts_at"`
	EndsAt      time.Time   `json:"ends_at"`
	Status      ShiftStatus `json:"status"`
	Counsellors []*User     `json:"counsellors,omitempty"`
	Timeslots   []*Timeslot `json:"timeslots,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type Timeslot struct {
	ID          uuid.UUID `json:"id"`
	ShiftID     uuid.UUID `json:"shift_id"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	IsAvailable bool      `json:"is_available"`
	CreatedAt   time.Time `json:"created_at"`
}
