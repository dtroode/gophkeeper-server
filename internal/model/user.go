package model

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// UserStore defines persistence operations for users.
type UserStore interface {
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByID(ctx context.Context, id uuid.UUID) (User, error)
	Create(ctx context.Context, user User) (User, error)
}

// User represents a stored user with authentication material.
type User struct {
	ID        uuid.UUID
	Email     string
	StoredKey []byte
	ServerKey []byte
	SaltRoot  []byte
	KDF       []byte
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}
