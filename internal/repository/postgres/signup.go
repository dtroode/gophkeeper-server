package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/dtroode/gophkeeper-server/internal/model"
)

var _ model.SignupStore = (*SignupRepository)(nil)

type SignupRepository struct {
	db *Connection
}

func NewSignupRepository(db *Connection) *SignupRepository {
	return &SignupRepository{
		db: db,
	}
}

func (r *SignupRepository) Create(ctx context.Context, pendingSignup model.PendingSignup) error {
	query := `INSERT INTO pending_signups (session_id, login, salt_root, kdf, expires_at, consumed)
			  VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.db.Exec(ctx, query,
		pendingSignup.SessionID, pendingSignup.Login, pendingSignup.SaltRoot,
		pendingSignup.KDF, pendingSignup.ExpiresAt, pendingSignup.Consumed,
	)
	if err != nil {
		return fmt.Errorf("failed to create pending signup: %w", err)
	}

	return nil
}

func (r *SignupRepository) GetBySessionID(ctx context.Context, sessionID string) (model.PendingSignup, error) {
	var pendingSignup model.PendingSignup
	query := `SELECT session_id, login, salt_root, kdf, expires_at, consumed
			  FROM pending_signups WHERE session_id = $1`

	err := r.db.QueryRow(ctx, query, sessionID).Scan(
		&pendingSignup.SessionID, &pendingSignup.Login, &pendingSignup.SaltRoot,
		&pendingSignup.KDF, &pendingSignup.ExpiresAt, &pendingSignup.Consumed,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.PendingSignup{}, fmt.Errorf("pending signup not found")
		}
		return model.PendingSignup{}, fmt.Errorf("failed to get pending signup by session id: %w", err)
	}

	return pendingSignup, nil
}

func (r *SignupRepository) Consume(ctx context.Context, sessionID string) error {
	query := `UPDATE pending_signups SET consumed = TRUE WHERE session_id = $1`

	_, err := r.db.Exec(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to consume signup session: %w", err)
	}

	return nil
}
