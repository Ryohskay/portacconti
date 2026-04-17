package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Ryohskay/portacconti/internal/domain"
	appcrypto "github.com/Ryohskay/portacconti/pkg/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepo struct {
	db  *pgxpool.Pool
	key string
}

func NewMessageRepo(db *pgxpool.Pool, encryptionKey string) *MessageRepo {
	return &MessageRepo{db: db, key: encryptionKey}
}

func (r *MessageRepo) Create(ctx context.Context, msg *domain.Message) error {
	subjectEnc, err := appcrypto.EncryptString(r.key, msg.Subject)
	if err != nil {
		return fmt.Errorf("encrypt subject: %w", err)
	}
	bodyEnc, err := appcrypto.EncryptString(r.key, msg.Body)
	if err != nil {
		return fmt.Errorf("encrypt body: %w", err)
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO messages (appointment_id, sender_id, recipient_id, subject_enc, body_enc)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, sent_at`,
		msg.AppointmentID, msg.SenderID, msg.RecipientID, subjectEnc, bodyEnc,
	)
	return row.Scan(&msg.ID, &msg.SentAt)
}

func (r *MessageRepo) ListByAppointment(ctx context.Context, appointmentID uuid.UUID) ([]*domain.Message, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, appointment_id, sender_id, recipient_id, subject_enc, body_enc, sent_at, read_at
		FROM messages WHERE appointment_id=$1 ORDER BY sent_at`, appointmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []*domain.Message
	for rows.Next() {
		msg := &domain.Message{}
		var subjectEnc, bodyEnc []byte
		var readAt *time.Time
		if err := rows.Scan(&msg.ID, &msg.AppointmentID, &msg.SenderID, &msg.RecipientID,
			&subjectEnc, &bodyEnc, &msg.SentAt, &readAt); err != nil {
			return nil, err
		}
		msg.ReadAt = readAt
		msg.Subject, err = appcrypto.DecryptString(r.key, subjectEnc)
		if err != nil {
			return nil, err
		}
		msg.Body, err = appcrypto.DecryptString(r.key, bodyEnc)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}

func (r *MessageRepo) MarkRead(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE messages SET read_at=NOW() WHERE id=$1 AND read_at IS NULL`, id)
	return err
}
