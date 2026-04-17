package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Ryohskay/portacconti/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ShiftRepo struct {
	db      *pgxpool.Pool
	userKey string
}

func NewShiftRepo(db *pgxpool.Pool, encryptionKey string) *ShiftRepo {
	return &ShiftRepo{db: db, userKey: encryptionKey}
}

func (r *ShiftRepo) CreateShift(ctx context.Context, s *domain.Shift) error {
	row := r.db.QueryRow(ctx, `
		INSERT INTO shifts (manager_id, starts_at, ends_at)
		VALUES ($1, $2, $3)
		RETURNING id, status, created_at, updated_at`,
		s.ManagerID, s.StartsAt, s.EndsAt,
	)
	return row.Scan(&s.ID, &s.Status, &s.CreatedAt, &s.UpdatedAt)
}

func (r *ShiftRepo) GetShiftByID(ctx context.Context, id uuid.UUID) (*domain.Shift, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, manager_id, starts_at, ends_at, status, created_at, updated_at
		FROM shifts WHERE id=$1`, id)
	s := &domain.Shift{}
	err := row.Scan(&s.ID, &s.ManagerID, &s.StartsAt, &s.EndsAt, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return s, err
}

func (r *ShiftRepo) ListShifts(ctx context.Context, from, to time.Time) ([]*domain.Shift, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, manager_id, starts_at, ends_at, status, created_at, updated_at
		FROM shifts WHERE starts_at >= $1 AND ends_at <= $2 ORDER BY starts_at`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var shifts []*domain.Shift
	for rows.Next() {
		s := &domain.Shift{}
		if err := rows.Scan(&s.ID, &s.ManagerID, &s.StartsAt, &s.EndsAt, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		shifts = append(shifts, s)
	}
	return shifts, rows.Err()
}

func (r *ShiftRepo) UpdateShift(ctx context.Context, s *domain.Shift) error {
	_, err := r.db.Exec(ctx, `
		UPDATE shifts SET starts_at=$1, ends_at=$2, status=$3, updated_at=NOW()
		WHERE id=$4`, s.StartsAt, s.EndsAt, string(s.Status), s.ID)
	return err
}

func (r *ShiftRepo) CloseShift(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE shifts SET status='closed', updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *ShiftRepo) CreateTimeslot(ctx context.Context, slot *domain.Timeslot) error {
	row := r.db.QueryRow(ctx, `
		INSERT INTO timeslots (shift_id, starts_at, ends_at)
		VALUES ($1, $2, $3)
		RETURNING id, is_available, created_at`,
		slot.ShiftID, slot.StartsAt, slot.EndsAt,
	)
	return row.Scan(&slot.ID, &slot.IsAvailable, &slot.CreatedAt)
}

func (r *ShiftRepo) GetTimeslotByID(ctx context.Context, id uuid.UUID) (*domain.Timeslot, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, shift_id, starts_at, ends_at, is_available, created_at
		FROM timeslots WHERE id=$1`, id)
	slot := &domain.Timeslot{}
	err := row.Scan(&slot.ID, &slot.ShiftID, &slot.StartsAt, &slot.EndsAt, &slot.IsAvailable, &slot.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return slot, err
}

func (r *ShiftRepo) ListAvailableTimeslots(ctx context.Context, from, to time.Time) ([]*domain.Timeslot, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, shift_id, starts_at, ends_at, is_available, created_at
		FROM timeslots
		WHERE is_available=TRUE AND starts_at >= $1 AND ends_at <= $2
		ORDER BY starts_at`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTimeslots(rows)
}

func (r *ShiftRepo) ListTimeslotsByShift(ctx context.Context, shiftID uuid.UUID) ([]*domain.Timeslot, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, shift_id, starts_at, ends_at, is_available, created_at
		FROM timeslots WHERE shift_id=$1 ORDER BY starts_at`, shiftID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTimeslots(rows)
}

func (r *ShiftRepo) SetTimeslotAvailability(ctx context.Context, id uuid.UUID, available bool) error {
	_, err := r.db.Exec(ctx, `UPDATE timeslots SET is_available=$1 WHERE id=$2`, available, id)
	return err
}

func (r *ShiftRepo) AddCounsellorToShift(ctx context.Context, shiftID, counsellorID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO shift_counsellors (shift_id, counsellor_id)
		VALUES ($1, $2) ON CONFLICT DO NOTHING`, shiftID, counsellorID)
	return err
}

func (r *ShiftRepo) RemoveCounsellorFromShift(ctx context.Context, shiftID, counsellorID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM shift_counsellors WHERE shift_id=$1 AND counsellor_id=$2`, shiftID, counsellorID)
	return err
}

func (r *ShiftRepo) GetShiftCounsellors(ctx context.Context, shiftID uuid.UUID) ([]*domain.User, error) {
	userRepo := NewUserRepo(r.db, r.userKey)
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.email, u.password_hash, u.role, u.name_enc, u.phone_enc, u.dob_enc, u.locale, u.is_active, u.created_at, u.updated_at
		FROM users u
		JOIN shift_counsellors sc ON sc.counsellor_id = u.id
		WHERE sc.shift_id=$1 AND u.deleted_at IS NULL`, shiftID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return userRepo.scanUsers(rows)
}

// FindAvailableCounsellorsForSlot returns counsellors who:
// 1. Are assigned to the shift that owns this timeslot.
// 2. Don't already have a confirmed/in-progress appointment that overlaps.
func (r *ShiftRepo) FindAvailableCounsellorsForSlot(ctx context.Context, slotID uuid.UUID) ([]*domain.User, error) {
	userRepo := NewUserRepo(r.db, r.userKey)
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT u.id, u.email, u.password_hash, u.role, u.name_enc, u.phone_enc, u.dob_enc, u.locale, u.is_active, u.created_at, u.updated_at
		FROM users u
		JOIN shift_counsellors sc ON sc.counsellor_id = u.id
		JOIN shifts sh ON sh.id = sc.shift_id
		JOIN timeslots ts ON ts.id = $1
		WHERE ts.shift_id = sh.id
		  AND u.role = 'counsellor'
		  AND u.is_active = TRUE
		  AND u.deleted_at IS NULL
		  AND u.id NOT IN (
			  SELECT a.counsellor_id
			  FROM appointments a
			  JOIN timeslots ats ON ats.id = a.timeslot_id
			  WHERE a.status IN ('confirmed', 'in_progress')
			    AND a.counsellor_id IS NOT NULL
			    AND ats.starts_at < ts.ends_at
			    AND ats.ends_at > ts.starts_at
		  )
		ORDER BY u.created_at`, slotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return userRepo.scanUsers(rows)
}

func scanTimeslots(rows pgx.Rows) ([]*domain.Timeslot, error) {
	var slots []*domain.Timeslot
	for rows.Next() {
		s := &domain.Timeslot{}
		if err := rows.Scan(&s.ID, &s.ShiftID, &s.StartsAt, &s.EndsAt, &s.IsAvailable, &s.CreatedAt); err != nil {
			return nil, err
		}
		slots = append(slots, s)
	}
	return slots, rows.Err()
}
