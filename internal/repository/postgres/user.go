package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dtroode/gophkeeper-server/internal/model"
)

var _ model.UserStore = (*UserRepository)(nil)

type UserRepository struct {
	db *Connection
}

func NewUserRepository(db *Connection) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (model.User, error) {
	var user model.User
	query := `SELECT id, email, stored_key, server_key, salt_root, kdf, created_at, updated_at, deleted_at
			  FROM users WHERE email = $1 AND deleted_at IS NULL`

	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.StoredKey, &user.ServerKey, &user.SaltRoot, &user.KDF,
		&user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, model.ErrNotFound
		}
		return model.User{}, fmt.Errorf("failed to get user by email: %w", err)
	}

	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (model.User, error) {
	var user model.User
	query := `SELECT id, email, stored_key, server_key, salt_root, kdf, created_at, updated_at, deleted_at
			  FROM users WHERE id = $1 AND deleted_at IS NULL`

	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.StoredKey, &user.ServerKey, &user.SaltRoot, &user.KDF,
		&user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, model.ErrNotFound
		}
		return model.User{}, fmt.Errorf("failed to get user by id: %w", err)
	}

	return user, nil
}

func (r *UserRepository) Create(ctx context.Context, user model.User) (model.User, error) {
	query := `INSERT INTO users (id, email, stored_key, server_key, salt_root, kdf, created_at, updated_at)
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			  RETURNING id, email, stored_key, server_key, salt_root, kdf, created_at, updated_at, deleted_at`

	var savedUser model.User
	err := r.db.QueryRow(ctx, query,
		user.ID, user.Email, user.StoredKey, user.ServerKey, user.SaltRoot, user.KDF,
		user.CreatedAt, user.UpdatedAt,
	).Scan(
		&savedUser.ID, &savedUser.Email, &savedUser.StoredKey, &savedUser.ServerKey,
		&savedUser.SaltRoot, &savedUser.KDF, &savedUser.CreatedAt, &savedUser.UpdatedAt, &savedUser.DeletedAt,
	)
	if err != nil {
		return model.User{}, fmt.Errorf("failed to create user: %w", err)
	}

	return savedUser, nil
}
