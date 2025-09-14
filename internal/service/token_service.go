package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dtroode/gophkeeper-server/internal/logger"
	"github.com/dtroode/gophkeeper-server/internal/model"
)

// TokenService provides high-level operations for issuing, refreshing,
// and revoking tokens. It composes the TokenManager and RefreshTokenStore.
type TokenService struct {
	manager model.TokenManager
	store   model.RefreshTokenStore
	logger  *logger.Logger
}

func NewTokenService(manager model.TokenManager, store model.RefreshTokenStore, logger *logger.Logger) *TokenService {
	return &TokenService{manager: manager, store: store, logger: logger}
}

// NOTE: Keep durations here in sync with the token manager. These are used
// only for persistence (cleanup/queries); cryptographic validity is checked
// against the JWT claims by the manager at parse time.
const (
	refreshTTL = 30 * 24 * time.Hour
)

func (s *TokenService) Issue(ctx context.Context, userID uuid.UUID) (accessToken string, refreshToken string, err error) {
	access, err := s.manager.GenerateAccessToken(userID)
	if err != nil {
		return "", "", fmt.Errorf("issue access: %w", err)
	}

	refresh, jti, err := s.manager.GenerateRefreshToken(userID)
	if err != nil {
		return "", "", fmt.Errorf("issue refresh: %w", err)
	}

	now := time.Now()
	rt := model.RefreshToken{
		ID:        uuid.New(),
		JTI:       jti,
		UserID:    userID,
		TokenHash: hashRefresh(refresh),
		IssuedAt:  now,
		ExpiresAt: now.Add(refreshTTL),
		RevokedAt: nil,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.store.Create(ctx, rt); err != nil {
		return "", "", fmt.Errorf("persist refresh: %w", err)
	}

	return access, refresh, nil
}

func (s *TokenService) Refresh(ctx context.Context, presentedRefresh string) (newAccess string, newRefresh string, err error) {
	userID, jti, err := s.manager.ParseRefreshToken(presentedRefresh)
	if err != nil {
		return "", "", err
	}

	rt, err := s.store.GetByJTI(ctx, jti)
	if err != nil {
		return "", "", err
	}

	// Validate stored state vs presented token.
	if err := validateRecord(rt, hashRefresh(presentedRefresh), time.Now()); err != nil {
		return "", "", err
	}

	// Revoke old token (rotation) and issue new pair.
	if err := s.store.RevokeByJTI(ctx, jti); err != nil {
		return "", "", fmt.Errorf("revoke old refresh: %w", err)
	}

	access, err := s.manager.GenerateAccessToken(userID)
	if err != nil {
		return "", "", fmt.Errorf("issue new access: %w", err)
	}

	refresh, newJTI, err := s.manager.GenerateRefreshToken(userID)
	if err != nil {
		return "", "", fmt.Errorf("issue new refresh: %w", err)
	}

	now := time.Now()
	rotatedFrom := rt.JTI
	newRT := model.RefreshToken{
		ID:             uuid.New(),
		JTI:            newJTI,
		UserID:         userID,
		TokenHash:      hashRefresh(refresh),
		IssuedAt:       now,
		ExpiresAt:      now.Add(refreshTTL),
		RevokedAt:      nil,
		RotatedFromJTI: &rotatedFrom,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.store.Create(ctx, newRT); err != nil {
		return "", "", fmt.Errorf("persist new refresh: %w", err)
	}

	return access, refresh, nil
}

func (s *TokenService) RevokeByToken(ctx context.Context, presentedRefresh string) error {
	_, jti, err := s.manager.ParseRefreshToken(presentedRefresh)
	if err != nil {
		return err
	}
	return s.store.RevokeByJTI(ctx, jti)
}

func (s *TokenService) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	return s.store.RevokeAllByUser(ctx, userID)
}

func (s *TokenService) GetUserID(ctx context.Context, token string) (uuid.UUID, error) {
	return s.manager.ParseAccessToken(token)
}

func hashRefresh(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

func validateRecord(rt model.RefreshToken, presentedHash []byte, now time.Time) error {
	if rt.RevokedAt != nil {
		return model.ErrTokenRevoked
	}
	if now.After(rt.ExpiresAt) {
		return model.ErrTokenExpired
	}
	if !equalBytes(rt.TokenHash, presentedHash) {
		return model.ErrTokenMismatch
	}
	return nil
}

func equalBytes(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
