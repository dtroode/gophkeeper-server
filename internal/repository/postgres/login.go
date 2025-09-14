package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/dtroode/gophkeeper-server/internal/model"
)

// Ensure LoginRepository implements the model.LoginStore interface.
var _ model.LoginStore = (*LoginRepository)(nil)

type LoginRepository struct {
	db *Connection
}

func NewLoginRepository(db *Connection) *LoginRepository {
	return &LoginRepository{db: db}
}

func (r *LoginRepository) Create(ctx context.Context, pendingLogin model.PendingLogin) error {
	const query = `
        INSERT INTO pending_logins (session_id, login, client_nonce, server_nonce, expires_at, consumed)
        VALUES ($1, $2, $3, $4, $5, $6)
    `

	if _, err := r.db.Exec(ctx, query,
		pendingLogin.SessionID,
		pendingLogin.Login,
		pendingLogin.ClientNonce,
		pendingLogin.ServerNonce,
		pendingLogin.ExpiresAt,
		pendingLogin.Consumed,
	); err != nil {
		return fmt.Errorf("failed to create pending login: %w", err)
	}
	return nil
}

func (r *LoginRepository) GetBySessionID(ctx context.Context, sessionID string) (model.PendingLogin, error) {
	const query = `
        SELECT session_id, login, client_nonce, server_nonce, expires_at, consumed
        FROM pending_logins
        WHERE session_id = $1
    `
	var pl model.PendingLogin
	if err := r.db.QueryRow(ctx, query, sessionID).Scan(
		&pl.SessionID,
		&pl.Login,
		&pl.ClientNonce,
		&pl.ServerNonce,
		&pl.ExpiresAt,
		&pl.Consumed,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.PendingLogin{}, fmt.Errorf("pending login not found")
		}
		return model.PendingLogin{}, fmt.Errorf("failed to get pending login by session id: %w", err)
	}
	return pl, nil
}

func (r *LoginRepository) Consume(ctx context.Context, sessionID string) error {
	const query = `
        UPDATE pending_logins
        SET consumed = TRUE
        WHERE session_id = $1
    `
	if _, err := r.db.Exec(ctx, query, sessionID); err != nil {
		return fmt.Errorf("failed to consume login session: %w", err)
	}
	return nil
}
