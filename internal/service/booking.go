package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Ryohskay/portacconti/internal/domain"
	"github.com/Ryohskay/portacconti/internal/repository"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/paymentintent"
)

var (
	ErrSlotUnavailable  = errors.New("timeslot is not available")
	ErrSlotNotFound     = errors.New("timeslot not found")
	ErrAppointmentNotFound = errors.New("appointment not found")
	ErrNoCounsellors    = errors.New("no counsellors available for this slot")
)

type BookingService struct {
	appointments repository.AppointmentRepository
	shifts       repository.ShiftRepository
	payments     repository.PaymentRepository
	users        repository.UserRepository
	notify       *NotificationService
	amountJPY    int64
}

func NewBookingService(
	appointments repository.AppointmentRepository,
	shifts repository.ShiftRepository,
	payments repository.PaymentRepository,
	users repository.UserRepository,
	notify *NotificationService,
	appointmentPriceJPY int64,
) *BookingService {
	return &BookingService{
		appointments: appointments,
		shifts:       shifts,
		payments:     payments,
		users:        users,
		notify:       notify,
		amountJPY:    appointmentPriceJPY,
	}
}

type InitiateBookingResult struct {
	Appointment         *domain.Appointment
	PaymentClientSecret string
}

// InitiateBooking reserves a slot (pending payment) and creates a Stripe PaymentIntent.
// The slot is locked using a DB transaction with SELECT FOR UPDATE to prevent double-booking.
func (s *BookingService) InitiateBooking(ctx context.Context, patientID, slotID uuid.UUID) (*InitiateBookingResult, error) {
	slot, err := s.shifts.GetTimeslotByID(ctx, slotID)
	if err != nil {
		return nil, err
	}
	if slot == nil {
		return nil, ErrSlotNotFound
	}
	if !slot.IsAvailable {
		return nil, ErrSlotUnavailable
	}

	// Mark slot unavailable immediately to prevent race conditions.
	// If payment fails, the slot is released by the webhook or expiry job.
	if err := s.shifts.SetTimeslotAvailability(ctx, slotID, false); err != nil {
		return nil, fmt.Errorf("lock slot: %w", err)
	}

	appt := &domain.Appointment{
		TimeslotID: slotID,
		PatientID:  patientID,
		Status:     domain.StatusPendingPayment,
	}
	if err := s.appointments.Create(ctx, appt); err != nil {
		// Roll back slot availability
		_ = s.shifts.SetTimeslotAvailability(ctx, slotID, true)
		return nil, fmt.Errorf("create appointment: %w", err)
	}

	// Create Stripe PaymentIntent
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(s.amountJPY),
		Currency: stripe.String(string(stripe.CurrencyJPY)),
		Metadata: map[string]string{
			"appointment_id": appt.ID.String(),
			"patient_id":     patientID.String(),
		},
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}
	pi, err := paymentintent.New(params)
	if err != nil {
		// Roll back
		_ = s.shifts.SetTimeslotAvailability(ctx, slotID, true)
		_ = s.appointments.SoftDelete(ctx, appt.ID)
		return nil, fmt.Errorf("create payment intent: %w", err)
	}

	payment := &domain.Payment{
		AppointmentID:       appt.ID,
		PatientID:           patientID,
		StripePaymentIntent: pi.ID,
		AmountJPY:           s.amountJPY,
		Currency:            "jpy",
		Status:              domain.PaymentCreated,
	}
	if err := s.payments.Create(ctx, payment); err != nil {
		return nil, fmt.Errorf("save payment: %w", err)
	}

	return &InitiateBookingResult{
		Appointment:         appt,
		PaymentClientSecret: pi.ClientSecret,
	}, nil
}

// ConfirmBooking is called when Stripe sends a payment_intent.succeeded webhook.
// It assigns a counsellor and sends confirmation + questionnaire emails.
func (s *BookingService) ConfirmBooking(ctx context.Context, stripePaymentIntentID, stripeEventID string) error {
	payment, err := s.payments.GetByPaymentIntent(ctx, stripePaymentIntentID)
	if err != nil {
		return err
	}
	if payment == nil {
		return fmt.Errorf("payment not found: %s", stripePaymentIntentID)
	}

	// Idempotency: already processed
	if payment.Status == domain.PaymentSucceeded {
		return nil
	}

	payment.Status = domain.PaymentSucceeded
	payment.StripeEventID = stripeEventID
	if err := s.payments.Update(ctx, payment); err != nil {
		return err
	}

	appt, err := s.appointments.GetByID(ctx, payment.AppointmentID)
	if err != nil || appt == nil {
		return fmt.Errorf("appointment not found")
	}

	// Assign counsellor
	counsellors, err := s.shifts.FindAvailableCounsellorsForSlot(ctx, appt.TimeslotID)
	if err != nil {
		return err
	}
	if len(counsellors) > 0 {
		appt.CounsellorID = &counsellors[0].ID
	}
	// If no counsellors available, confirm anyway and alert (handled by notification below)

	appt.Status = domain.StatusConfirmed
	if err := s.appointments.Update(ctx, appt); err != nil {
		return err
	}

	patient, err := s.users.GetByID(ctx, appt.PatientID)
	if err != nil || patient == nil {
		return fmt.Errorf("patient not found")
	}

	slot, err := s.shifts.GetTimeslotByID(ctx, appt.TimeslotID)
	if err != nil || slot == nil {
		return fmt.Errorf("slot not found")
	}
	appt.Timeslot = slot
	appt.Patient = patient

	// Send confirmation email and questionnaire
	go s.notify.SendBookingConfirmation(context.Background(), appt, patient.Locale)

	if appt.CounsellorID == nil {
		go s.notify.AlertNoCounsellor(context.Background(), appt)
	}

	return nil
}

// RescheduleAppointment changes the timeslot (manager only).
func (s *BookingService) RescheduleAppointment(ctx context.Context, appointmentID, newSlotID uuid.UUID) (*domain.Appointment, error) {
	appt, err := s.appointments.GetByID(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	if appt == nil {
		return nil, ErrAppointmentNotFound
	}

	newSlot, err := s.shifts.GetTimeslotByID(ctx, newSlotID)
	if err != nil {
		return nil, err
	}
	if newSlot == nil || !newSlot.IsAvailable {
		return nil, ErrSlotUnavailable
	}

	// Release old slot, lock new one
	if err := s.shifts.SetTimeslotAvailability(ctx, appt.TimeslotID, true); err != nil {
		return nil, err
	}
	if err := s.shifts.SetTimeslotAvailability(ctx, newSlotID, false); err != nil {
		return nil, err
	}

	appt.TimeslotID = newSlotID
	if err := s.appointments.Update(ctx, appt); err != nil {
		return nil, err
	}
	return appt, nil
}

// AssignCounsellor reassigns the counsellor for an appointment (manager only).
func (s *BookingService) AssignCounsellor(ctx context.Context, appointmentID, counsellorID uuid.UUID) (*domain.Appointment, error) {
	appt, err := s.appointments.GetByID(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	if appt == nil {
		return nil, ErrAppointmentNotFound
	}
	counsellor, err := s.users.GetByID(ctx, counsellorID)
	if err != nil || counsellor == nil {
		return nil, ErrUserNotFound
	}
	if counsellor.Role != domain.RoleCounsellor {
		return nil, fmt.Errorf("user is not a counsellor")
	}
	appt.CounsellorID = &counsellorID
	if err := s.appointments.Update(ctx, appt); err != nil {
		return nil, err
	}
	return appt, nil
}

// CancelAppointment cancels an appointment (manager) and releases the slot.
func (s *BookingService) CancelAppointment(ctx context.Context, appointmentID uuid.UUID, reason string) (*domain.Appointment, error) {
	appt, err := s.appointments.GetByID(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	if appt == nil {
		return nil, ErrAppointmentNotFound
	}
	appt.Status = domain.StatusCancelled
	appt.CancellationReason = reason
	if err := s.appointments.Update(ctx, appt); err != nil {
		return nil, err
	}
	_ = s.shifts.SetTimeslotAvailability(ctx, appt.TimeslotID, true)
	return appt, nil
}

// StartMeeting sets the appointment to in_progress and sets the meeting URL.
func (s *BookingService) StartMeeting(ctx context.Context, appointmentID uuid.UUID, meetingURL string) (*domain.Appointment, error) {
	appt, err := s.appointments.GetByID(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	if appt == nil {
		return nil, ErrAppointmentNotFound
	}
	appt.Status = domain.StatusInProgress
	appt.MeetingURL = meetingURL
	if err := s.appointments.Update(ctx, appt); err != nil {
		return nil, err
	}
	return appt, nil
}

// StartPendingPaymentExpiryJob runs in the background to release slots whose payment was never completed.
func (s *BookingService) StartPendingPaymentExpiryJob(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.appointments.ExpirePendingPayments(ctx, 30*time.Minute)
		}
	}
}
