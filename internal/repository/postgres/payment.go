package postgres

import (
	"context"
	"errors"

	"github.com/Ryohskay/portacconti/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentRepo struct {
	db *pgxpool.Pool
}

func NewPaymentRepo(db *pgxpool.Pool) *PaymentRepo {
	return &PaymentRepo{db: db}
}

func (r *PaymentRepo) Create(ctx context.Context, p *domain.Payment) error {
	row := r.db.QueryRow(ctx, `
		INSERT INTO payments (appointment_id, patient_id, stripe_payment_intent, amount_jpy, currency, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`,
		p.AppointmentID, p.PatientID, p.StripePaymentIntent, p.AmountJPY, p.Currency, string(p.Status),
	)
	return row.Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *PaymentRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, appointment_id, patient_id, stripe_payment_intent, amount_jpy, currency, status, COALESCE(stripe_event_id,''), created_at, updated_at
		FROM payments WHERE id=$1`, id)
	return scanPayment(row)
}

func (r *PaymentRepo) GetByPaymentIntent(ctx context.Context, intentID string) (*domain.Payment, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, appointment_id, patient_id, stripe_payment_intent, amount_jpy, currency, status, COALESCE(stripe_event_id,''), created_at, updated_at
		FROM payments WHERE stripe_payment_intent=$1`, intentID)
	return scanPayment(row)
}

func (r *PaymentRepo) Update(ctx context.Context, p *domain.Payment) error {
	_, err := r.db.Exec(ctx, `
		UPDATE payments SET status=$1, stripe_event_id=$2, updated_at=NOW() WHERE id=$3`,
		string(p.Status), p.StripeEventID, p.ID,
	)
	return err
}

func scanPayment(row pgx.Row) (*domain.Payment, error) {
	p := &domain.Payment{}
	err := row.Scan(
		&p.ID, &p.AppointmentID, &p.PatientID,
		&p.StripePaymentIntent, &p.AmountJPY, &p.Currency,
		&p.Status, &p.StripeEventID,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}
