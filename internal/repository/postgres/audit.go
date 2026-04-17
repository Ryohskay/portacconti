package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRepo struct {
	db *pgxpool.Pool
}

func NewAuditRepo(db *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{db: db}
}

func (r *AuditRepo) Log(ctx context.Context, actorID *uuid.UUID, action, targetType string, targetID *uuid.UUID, ip string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO audit_log (actor_id, action, target_type, target_id, ip_address)
		VALUES ($1, $2, $3, $4, $5::INET)`,
		actorID, action, targetType, targetID, ip,
	)
	return err
}
