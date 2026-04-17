package repository

import (
	"context"
	"time"

	"github.com/Ryohskay/portacconti/internal/domain"
	"github.com/google/uuid"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListByRole(ctx context.Context, role domain.Role) ([]*domain.User, error)

	SaveRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error
	GetRefreshToken(ctx context.Context, tokenHash string) (uuid.UUID, bool, error) // returns userID, valid
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeAllRefreshTokens(ctx context.Context, userID uuid.UUID) error
}

type ShiftRepository interface {
	CreateShift(ctx context.Context, shift *domain.Shift) error
	GetShiftByID(ctx context.Context, id uuid.UUID) (*domain.Shift, error)
	ListShifts(ctx context.Context, from, to time.Time) ([]*domain.Shift, error)
	UpdateShift(ctx context.Context, shift *domain.Shift) error
	CloseShift(ctx context.Context, id uuid.UUID) error

	CreateTimeslot(ctx context.Context, slot *domain.Timeslot) error
	GetTimeslotByID(ctx context.Context, id uuid.UUID) (*domain.Timeslot, error)
	ListAvailableTimeslots(ctx context.Context, from, to time.Time) ([]*domain.Timeslot, error)
	ListTimeslotsByShift(ctx context.Context, shiftID uuid.UUID) ([]*domain.Timeslot, error)
	SetTimeslotAvailability(ctx context.Context, id uuid.UUID, available bool) error

	AddCounsellorToShift(ctx context.Context, shiftID, counsellorID uuid.UUID) error
	RemoveCounsellorFromShift(ctx context.Context, shiftID, counsellorID uuid.UUID) error
	GetShiftCounsellors(ctx context.Context, shiftID uuid.UUID) ([]*domain.User, error)
	FindAvailableCounsellorsForSlot(ctx context.Context, slotID uuid.UUID) ([]*domain.User, error)
}

type AppointmentRepository interface {
	Create(ctx context.Context, appt *domain.Appointment) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Appointment, error)
	Update(ctx context.Context, appt *domain.Appointment) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListByPatient(ctx context.Context, patientID uuid.UUID) ([]*domain.Appointment, error)
	ListByCounsellor(ctx context.Context, counsellorID uuid.UUID) ([]*domain.Appointment, error)
	ListAll(ctx context.Context) ([]*domain.Appointment, error)
	ExpirePendingPayments(ctx context.Context, olderThan time.Duration) error

	AddRecord(ctx context.Context, record *domain.PatientRecord) error
	GetRecord(ctx context.Context, id uuid.UUID) (*domain.PatientRecord, error)
	UpdateRecord(ctx context.Context, record *domain.PatientRecord) error
	ListRecords(ctx context.Context, appointmentID uuid.UUID) ([]*domain.PatientRecord, error)
}

type PaymentRepository interface {
	Create(ctx context.Context, payment *domain.Payment) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error)
	GetByPaymentIntent(ctx context.Context, intentID string) (*domain.Payment, error)
	Update(ctx context.Context, payment *domain.Payment) error
}

type MessageRepository interface {
	Create(ctx context.Context, msg *domain.Message) error
	ListByAppointment(ctx context.Context, appointmentID uuid.UUID) ([]*domain.Message, error)
	MarkRead(ctx context.Context, id uuid.UUID) error
}

type QuestionnaireRepository interface {
	GetActiveTemplate(ctx context.Context, locale string) (*domain.QuestionnaireTemplate, error)
	SaveToken(ctx context.Context, token *domain.QuestionnaireToken) error
	GetToken(ctx context.Context, tokenHash string) (*domain.QuestionnaireToken, error)
	MarkTokenUsed(ctx context.Context, tokenHash string) error
	SaveResponse(ctx context.Context, resp *domain.QuestionnaireResponse) error
	GetResponseByAppointment(ctx context.Context, appointmentID uuid.UUID) (*domain.QuestionnaireResponse, error)
}

type AuditRepository interface {
	Log(ctx context.Context, actorID *uuid.UUID, action, targetType string, targetID *uuid.UUID, ip string) error
}
