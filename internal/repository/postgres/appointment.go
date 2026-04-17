package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Ryohskay/portacconti/internal/domain"
	appcrypto "github.com/Ryohskay/portacconti/pkg/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AppointmentRepo struct {
	db  *pgxpool.Pool
	key string
}

func NewAppointmentRepo(db *pgxpool.Pool, encryptionKey string) *AppointmentRepo {
	return &AppointmentRepo{db: db, key: encryptionKey}
}

func (r *AppointmentRepo) Create(ctx context.Context, a *domain.Appointment) error {
	var cID pgtype.UUID
	if a.CounsellorID != nil {
		cID = pgtype.UUID{Bytes: *a.CounsellorID, Valid: true}
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO appointments (timeslot_id, patient_id, counsellor_id)
		VALUES ($1, $2, $3)
		RETURNING id, status, created_at, updated_at`,
		a.TimeslotID, a.PatientID, cID,
	)
	return row.Scan(&a.ID, &a.Status, &a.CreatedAt, &a.UpdatedAt)
}

func (r *AppointmentRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Appointment, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, timeslot_id, patient_id, counsellor_id, status, meeting_url, cancellation_reason, created_at, updated_at
		FROM appointments WHERE id=$1 AND deleted_at IS NULL`, id)
	return r.scanAppointment(row)
}

func (r *AppointmentRepo) Update(ctx context.Context, a *domain.Appointment) error {
	var cID pgtype.UUID
	if a.CounsellorID != nil {
		cID = pgtype.UUID{Bytes: *a.CounsellorID, Valid: true}
	}
	_, err := r.db.Exec(ctx, `
		UPDATE appointments
		SET timeslot_id=$1, counsellor_id=$2, status=$3, meeting_url=$4, cancellation_reason=$5, updated_at=NOW()
		WHERE id=$6`,
		a.TimeslotID, cID, string(a.Status), a.MeetingURL, a.CancellationReason, a.ID,
	)
	return err
}

func (r *AppointmentRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE appointments SET deleted_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *AppointmentRepo) ListByPatient(ctx context.Context, patientID uuid.UUID) ([]*domain.Appointment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, timeslot_id, patient_id, counsellor_id, status, meeting_url, cancellation_reason, created_at, updated_at
		FROM appointments WHERE patient_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC`, patientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanAppointments(rows)
}

func (r *AppointmentRepo) ListByCounsellor(ctx context.Context, counsellorID uuid.UUID) ([]*domain.Appointment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, timeslot_id, patient_id, counsellor_id, status, meeting_url, cancellation_reason, created_at, updated_at
		FROM appointments WHERE counsellor_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC`, counsellorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanAppointments(rows)
}

func (r *AppointmentRepo) ListAll(ctx context.Context) ([]*domain.Appointment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, timeslot_id, patient_id, counsellor_id, status, meeting_url, cancellation_reason, created_at, updated_at
		FROM appointments WHERE deleted_at IS NULL ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanAppointments(rows)
}

func (r *AppointmentRepo) ExpirePendingPayments(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Get appointments to expire
	rows, err := tx.Query(ctx, `
		SELECT id, timeslot_id FROM appointments
		WHERE status='pending_payment' AND created_at < $1 AND deleted_at IS NULL`, cutoff)
	if err != nil {
		return err
	}

	type row struct {
		id, slotID uuid.UUID
	}
	var toExpire []row
	for rows.Next() {
		var e row
		if err := rows.Scan(&e.id, &e.slotID); err != nil {
			rows.Close()
			return err
		}
		toExpire = append(toExpire, e)
	}
	rows.Close()

	for _, e := range toExpire {
		if _, err := tx.Exec(ctx, `UPDATE appointments SET status='cancelled', updated_at=NOW() WHERE id=$1`, e.id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE timeslots SET is_available=TRUE WHERE id=$1`, e.slotID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *AppointmentRepo) AddRecord(ctx context.Context, rec *domain.PatientRecord) error {
	enc, err := appcrypto.EncryptString(r.key, rec.Content)
	if err != nil {
		return fmt.Errorf("encrypt record: %w", err)
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO patient_records (appointment_id, author_id, content_enc)
		VALUES ($1, $2, $3) RETURNING id, created_at, updated_at`,
		rec.AppointmentID, rec.AuthorID, enc,
	)
	return row.Scan(&rec.ID, &rec.CreatedAt, &rec.UpdatedAt)
}

func (r *AppointmentRepo) GetRecord(ctx context.Context, id uuid.UUID) (*domain.PatientRecord, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, appointment_id, author_id, content_enc, created_at, updated_at
		FROM patient_records WHERE id=$1`, id)
	return r.scanRecord(row)
}

func (r *AppointmentRepo) UpdateRecord(ctx context.Context, rec *domain.PatientRecord) error {
	enc, err := appcrypto.EncryptString(r.key, rec.Content)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		UPDATE patient_records SET content_enc=$1, updated_at=NOW() WHERE id=$2`, enc, rec.ID)
	return err
}

func (r *AppointmentRepo) ListRecords(ctx context.Context, appointmentID uuid.UUID) ([]*domain.PatientRecord, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, appointment_id, author_id, content_enc, created_at, updated_at
		FROM patient_records WHERE appointment_id=$1 ORDER BY created_at`, appointmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var recs []*domain.PatientRecord
	for rows.Next() {
		rec, err := r.scanRecord(rows)
		if err != nil {
			return nil, err
		}
		recs = append(recs, rec)
	}
	return recs, rows.Err()
}

func (r *AppointmentRepo) scanAppointment(row pgx.Row) (*domain.Appointment, error) {
	a := &domain.Appointment{}
	var cID pgtype.UUID
	err := row.Scan(
		&a.ID, &a.TimeslotID, &a.PatientID, &cID,
		&a.Status, &a.MeetingURL, &a.CancellationReason,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if cID.Valid {
		id := uuid.UUID(cID.Bytes)
		a.CounsellorID = &id
	}
	return a, nil
}

func (r *AppointmentRepo) scanAppointments(rows pgx.Rows) ([]*domain.Appointment, error) {
	var appts []*domain.Appointment
	for rows.Next() {
		a := &domain.Appointment{}
		var cID pgtype.UUID
		err := rows.Scan(
			&a.ID, &a.TimeslotID, &a.PatientID, &cID,
			&a.Status, &a.MeetingURL, &a.CancellationReason,
			&a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if cID.Valid {
			id := uuid.UUID(cID.Bytes)
			a.CounsellorID = &id
		}
		appts = append(appts, a)
	}
	return appts, rows.Err()
}

func (r *AppointmentRepo) scanRecord(row interface {
	Scan(...any) error
}) (*domain.PatientRecord, error) {
	rec := &domain.PatientRecord{}
	var enc []byte
	err := row.Scan(&rec.ID, &rec.AppointmentID, &rec.AuthorID, &enc, &rec.CreatedAt, &rec.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rec.Content, err = appcrypto.DecryptString(r.key, enc)
	return rec, err
}
