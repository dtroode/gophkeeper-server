package model

import (
	"context"
	"time"
)

// PendingSessionDuration is a TTL for pending sessions.
const PendingSessionDuration = time.Minute * 10

// SignupStore persists pending registration sessions.
type SignupStore interface {
	Create(ctx context.Context, pendingSignup PendingSignup) error
	GetBySessionID(ctx context.Context, sessionID string) (PendingSignup, error)
	Consume(ctx context.Context, sessionID string) error
}

// LoginStore persists pending login sessions.
type LoginStore interface {
	Create(ctx context.Context, pendingLogin PendingLogin) error
	GetBySessionID(ctx context.Context, sessionID string) (PendingLogin, error)
	Consume(ctx context.Context, sessionID string) error
}

// PendingSignup describes a pending registration session.
type PendingSignup struct {
	SessionID string
	Login     string
	SaltRoot  []byte
	KDF       []byte
	ExpiresAt time.Time
	Consumed  bool
}

// PendingLogin describes a pending login session.
type PendingLogin struct {
	SessionID   string
	Login       string
	ClientNonce []byte
	ServerNonce []byte
	ExpiresAt   time.Time
	Consumed    bool
}
