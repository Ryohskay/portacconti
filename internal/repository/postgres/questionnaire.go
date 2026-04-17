package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Ryohskay/portacconti/internal/domain"
	appcrypto "github.com/Ryohskay/portacconti/pkg/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QuestionnaireRepo struct {
	db  *pgxpool.Pool
	key string
}

func NewQuestionnaireRepo(db *pgxpool.Pool, encryptionKey string) *QuestionnaireRepo {
	return &QuestionnaireRepo{db: db, key: encryptionKey}
}

func (r *QuestionnaireRepo) GetActiveTemplate(ctx context.Context, locale string) (*domain.QuestionnaireTemplate, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, locale, title, schema_json, is_active, created_at
		FROM questionnaire_templates
		WHERE locale=$1 AND is_active=TRUE
		ORDER BY created_at DESC LIMIT 1`, locale)
	tmpl := &domain.QuestionnaireTemplate{}
	var schemaJSON []byte
	err := row.Scan(&tmpl.ID, &tmpl.Locale, &tmpl.Title, &schemaJSON, &tmpl.IsActive, &tmpl.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Fall back to English
		row = r.db.QueryRow(ctx, `
			SELECT id, locale, title, schema_json, is_active, created_at
			FROM questionnaire_templates WHERE is_active=TRUE ORDER BY created_at DESC LIMIT 1`)
		err = row.Scan(&tmpl.ID, &tmpl.Locale, &tmpl.Title, &schemaJSON, &tmpl.IsActive, &tmpl.CreatedAt)
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(schemaJSON, &tmpl.Questions); err != nil {
		return nil, fmt.Errorf("parse template schema: %w", err)
	}
	return tmpl, nil
}

func (r *QuestionnaireRepo) SaveToken(ctx context.Context, token *domain.QuestionnaireToken) error {
	row := r.db.QueryRow(ctx, `
		INSERT INTO questionnaire_tokens (appointment_id, template_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		token.AppointmentID, token.TemplateID, token.TokenHash, token.ExpiresAt,
	)
	return row.Scan(&token.ID)
}

func (r *QuestionnaireRepo) GetToken(ctx context.Context, tokenHash string) (*domain.QuestionnaireToken, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, appointment_id, template_id, token_hash, expires_at, used_at
		FROM questionnaire_tokens WHERE token_hash=$1`, tokenHash)
	tok := &domain.QuestionnaireToken{}
	var usedAt *time.Time
	err := row.Scan(&tok.ID, &tok.AppointmentID, &tok.TemplateID, &tok.TokenHash, &tok.ExpiresAt, &usedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	tok.UsedAt = usedAt
	return tok, err
}

func (r *QuestionnaireRepo) MarkTokenUsed(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx, `UPDATE questionnaire_tokens SET used_at=NOW() WHERE token_hash=$1`, tokenHash)
	return err
}

func (r *QuestionnaireRepo) SaveResponse(ctx context.Context, resp *domain.QuestionnaireResponse) error {
	answersJSON, err := json.Marshal(resp.Answers)
	if err != nil {
		return err
	}
	enc, err := appcrypto.Encrypt(r.key, answersJSON)
	if err != nil {
		return fmt.Errorf("encrypt answers: %w", err)
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO questionnaire_responses (appointment_id, template_id, answers_enc)
		VALUES ($1, $2, $3) RETURNING id, submitted_at`,
		resp.AppointmentID, resp.TemplateID, enc,
	)
	return row.Scan(&resp.ID, &resp.SubmittedAt)
}

func (r *QuestionnaireRepo) GetResponseByAppointment(ctx context.Context, appointmentID uuid.UUID) (*domain.QuestionnaireResponse, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, appointment_id, template_id, answers_enc, submitted_at
		FROM questionnaire_responses WHERE appointment_id=$1`, appointmentID)
	resp := &domain.QuestionnaireResponse{}
	var enc []byte
	err := row.Scan(&resp.ID, &resp.AppointmentID, &resp.TemplateID, &enc, &resp.SubmittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	plaintext, err := appcrypto.Decrypt(r.key, enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt answers: %w", err)
	}
	if err := json.Unmarshal(plaintext, &resp.Answers); err != nil {
		return nil, err
	}
	return resp, nil
}
