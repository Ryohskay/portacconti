package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RolePatient    Role = "patient"
	RoleCounsellor Role = "counsellor"
	RoleManager    Role = "manager"
)

type User struct {
	ID             uuid.UUID `json:"id"`
	Email          string    `json:"email"`
	HashedPassword string    `json:"-"`
	Name           string    `json:"name"`
	Phone          string    `json:"phone,omitempty"`
	DateOfBirth    *time.Time `json:"date_of_birth,omitempty"`
	Role           Role      `json:"role"`
	Locale         string    `json:"locale"` // "en" or "ja"
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
