package model

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type RefreshTokenStore interface {
	Create(ctx context.Context, token RefreshToken) error
	GetByJTI(ctx context.Context, jti string) (RefreshToken, error)
	RevokeByJTI(ctx context.Context, jti string) error
	RevokeAllByUser(ctx context.Context, userID uuid.UUID) error
}

type RefreshToken struct {
	ID             uuid.UUID
	JTI            string
	UserID         uuid.UUID
	TokenHash      []byte
	IssuedAt       time.Time
	ExpiresAt      time.Time
	RevokedAt      *time.Time
	RotatedFromJTI *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
