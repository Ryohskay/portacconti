package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/Ryohskay/portacconti/internal/domain"
	appcrypto "github.com/Ryohskay/portacconti/pkg/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct {
	db  *pgxpool.Pool
	key string // AES encryption key (hex)
}

func NewUserRepo(db *pgxpool.Pool, encryptionKey string) *UserRepo {
	return &UserRepo{db: db, key: encryptionKey}
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	nameEnc, err := appcrypto.EncryptString(r.key, u.Name)
	if err != nil {
		return fmt.Errorf("encrypt name: %w", err)
	}
	phoneEnc, err := appcrypto.EncryptString(r.key, u.Phone)
	if err != nil {
		return fmt.Errorf("encrypt phone: %w", err)
	}

	var dobEnc []byte
	if u.DateOfBirth != nil {
		dobEnc, err = appcrypto.EncryptString(r.key, u.DateOfBirth.Format(time.DateOnly))
		if err != nil {
			return fmt.Errorf("encrypt dob: %w", err)
		}
	}

	row := r.db.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, name_enc, phone_enc, dob_enc, locale)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		u.Email, u.HashedPassword, string(u.Role), nameEnc, phoneEnc, dobEnc, u.Locale,
	)
	return row.Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, role, name_enc, phone_enc, dob_enc, locale, is_active, created_at, updated_at
		FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
	return r.scanUser(row)
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, role, name_enc, phone_enc, dob_enc, locale, is_active, created_at, updated_at
		FROM users WHERE email = $1 AND deleted_at IS NULL`, email)
	return r.scanUser(row)
}

func (r *UserRepo) Update(ctx context.Context, u *domain.User) error {
	nameEnc, err := appcrypto.EncryptString(r.key, u.Name)
	if err != nil {
		return err
	}
	phoneEnc, err := appcrypto.EncryptString(r.key, u.Phone)
	if err != nil {
		return err
	}
	var dobEnc []byte
	if u.DateOfBirth != nil {
		dobEnc, err = appcrypto.EncryptString(r.key, u.DateOfBirth.Format(time.DateOnly))
		if err != nil {
			return err
		}
	}

	_, err = r.db.Exec(ctx, `
		UPDATE users SET name_enc=$1, phone_enc=$2, dob_enc=$3, locale=$4, is_active=$5, updated_at=NOW()
		WHERE id=$6`,
		nameEnc, phoneEnc, dobEnc, u.Locale, u.IsActive, u.ID,
	)
	return err
}

func (r *UserRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET deleted_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *UserRepo) ListByRole(ctx context.Context, role domain.Role) ([]*domain.User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, email, password_hash, role, name_enc, phone_enc, dob_enc, locale, is_active, created_at, updated_at
		FROM users WHERE role=$1 AND deleted_at IS NULL ORDER BY created_at`, string(role))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanUsers(rows)
}

func (r *UserRepo) SaveRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	return err
}

func (r *UserRepo) GetRefreshToken(ctx context.Context, tokenHash string) (uuid.UUID, bool, error) {
	var userID uuid.UUID
	var revoked bool
	var expiresAt time.Time
	err := r.db.QueryRow(ctx, `
		SELECT user_id, revoked, expires_at FROM refresh_tokens WHERE token_hash=$1`, tokenHash,
	).Scan(&userID, &revoked, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	if revoked || time.Now().After(expiresAt) {
		return uuid.Nil, false, nil
	}
	return userID, true, nil
}

func (r *UserRepo) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx, `UPDATE refresh_tokens SET revoked=TRUE WHERE token_hash=$1`, tokenHash)
	return err
}

func (r *UserRepo) RevokeAllRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE refresh_tokens SET revoked=TRUE WHERE user_id=$1`, userID)
	return err
}

func (r *UserRepo) scanUser(row pgx.Row) (*domain.User, error) {
	u := &domain.User{}
	var nameEnc, phoneEnc, dobEnc []byte
	err := row.Scan(
		&u.ID, &u.Email, &u.HashedPassword, &u.Role,
		&nameEnc, &phoneEnc, &dobEnc,
		&u.Locale, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.Name, err = appcrypto.DecryptString(r.key, nameEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt name: %w", err)
	}
	u.Phone, err = appcrypto.DecryptString(r.key, phoneEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt phone: %w", err)
	}
	if len(dobEnc) > 0 {
		dobStr, err2 := appcrypto.DecryptString(r.key, dobEnc)
		if err2 != nil {
			return nil, fmt.Errorf("decrypt dob: %w", err2)
		}
		dob, err2 := time.Parse(time.DateOnly, dobStr)
		if err2 == nil {
			u.DateOfBirth = &dob
		}
	}
	return u, nil
}

func (r *UserRepo) scanUsers(rows pgx.Rows) ([]*domain.User, error) {
	var users []*domain.User
	for rows.Next() {
		u := &domain.User{}
		var nameEnc, phoneEnc, dobEnc []byte
		err := rows.Scan(
			&u.ID, &u.Email, &u.HashedPassword, &u.Role,
			&nameEnc, &phoneEnc, &dobEnc,
			&u.Locale, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		u.Name, err = appcrypto.DecryptString(r.key, nameEnc)
		if err != nil {
			return nil, err
		}
		u.Phone, err = appcrypto.DecryptString(r.key, phoneEnc)
		if err != nil {
			return nil, err
		}
		if len(dobEnc) > 0 {
			dobStr, _ := appcrypto.DecryptString(r.key, dobEnc)
			dob, err2 := time.Parse(time.DateOnly, dobStr)
			if err2 == nil {
				u.DateOfBirth = &dob
			}
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// HashToken returns the SHA-256 hex hash of a token string.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
