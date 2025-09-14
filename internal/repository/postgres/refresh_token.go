package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dtroode/gophkeeper-server/internal/model"
)

var _ model.RefreshTokenStore = (*RefreshTokenRepository)(nil)

type RefreshTokenRepository struct {
	db *Connection
}

func NewRefreshTokenRepository(db *Connection) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, token model.RefreshToken) error {
	const query = `
        INSERT INTO refresh_tokens (
            id, jti, user_id, token_hash, issued_at, expires_at, revoked_at, rotated_from_jti, created_at, updated_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())
    `

	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}

	_, err := r.db.Exec(ctx, query,
		token.ID, token.JTI, token.UserID, token.TokenHash, token.IssuedAt, token.ExpiresAt,
		token.RevokedAt, token.RotatedFromJTI,
	)
	if err != nil {
		return fmt.Errorf("failed to create refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) GetByJTI(ctx context.Context, jti string) (model.RefreshToken, error) {
	const query = `
        SELECT id, jti, user_id, token_hash, issued_at, expires_at, revoked_at, rotated_from_jti, created_at, updated_at
        FROM refresh_tokens WHERE jti = $1
    `
	var rt model.RefreshToken
	err := r.db.QueryRow(ctx, query, jti).Scan(
		&rt.ID, &rt.JTI, &rt.UserID, &rt.TokenHash, &rt.IssuedAt, &rt.ExpiresAt,
		&rt.RevokedAt, &rt.RotatedFromJTI, &rt.CreatedAt, &rt.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.RefreshToken{}, model.ErrNotFound
		}
		return model.RefreshToken{}, fmt.Errorf("failed to get refresh token by jti: %w", err)
	}
	return rt, nil
}

func (r *RefreshTokenRepository) RevokeByJTI(ctx context.Context, jti string) error {
	const query = `
        UPDATE refresh_tokens SET revoked_at = NOW(), updated_at = NOW()
        WHERE jti = $1 AND revoked_at IS NULL
    `
	if _, err := r.db.Exec(ctx, query, jti); err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) RevokeAllByUser(ctx context.Context, userID uuid.UUID) error {
	const query = `
        UPDATE refresh_tokens SET revoked_at = NOW(), updated_at = NOW()
        WHERE user_id = $1 AND revoked_at IS NULL
    `
	if _, err := r.db.Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("failed to revoke refresh tokens by user: %w", err)
	}
	return nil
}
