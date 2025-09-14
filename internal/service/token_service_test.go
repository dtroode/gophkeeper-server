package service

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dtroode/gophkeeper-server/internal/logger"
	servermocks "github.com/dtroode/gophkeeper-server/internal/mocks"
	"github.com/dtroode/gophkeeper-server/internal/model"
)

func TestTokenService_Issue(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	manager := &servermocks.TokenManager{}
	store := &servermocks.RefreshTokenStore{}

	manager.On("GenerateAccessToken", userID).Return("access", nil).Once()
	manager.On("GenerateRefreshToken", userID).Return("refresh", "jti-1", nil).Once()
	store.On("Create", ctx, mock.Anything).Return(nil).Maybe()

	svc := NewTokenService(manager, store, logger.New(0))

	access, refresh, err := svc.Issue(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, "access", access)
	assert.Equal(t, "refresh", refresh)
}

func TestTokenService_Issue_ManagerError(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	manager := &servermocks.TokenManager{}
	store := &servermocks.RefreshTokenStore{}

	manager.On("GenerateAccessToken", userID).Return("", assert.AnError).Once()

	svc := NewTokenService(manager, store, logger.New(0))

	_, _, err := svc.Issue(ctx, userID)
	require.Error(t, err)
}

func TestTokenService_Refresh_Success(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	jti := "jti-old"
	presented := "refresh-old"
	h := sha256.Sum256([]byte(presented))
	presentedHash := h[:]

	manager := &servermocks.TokenManager{}
	store := &servermocks.RefreshTokenStore{}

	manager.On("ParseRefreshToken", presented).Return(userID, jti, nil).Once()
	store.On("GetByJTI", ctx, jti).Return(model.RefreshToken{
		JTI:       jti,
		UserID:    userID,
		TokenHash: presentedHash,
		IssuedAt:  time.Now().Add(-time.Hour),
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil).Once()
	store.On("RevokeByJTI", ctx, jti).Return(nil).Once()
	manager.On("GenerateAccessToken", userID).Return("access-new", nil).Once()
	manager.On("GenerateRefreshToken", userID).Return("refresh-new", "jti-new", nil).Once()
	store.On("Create", ctx, mock.Anything).Return(nil).Maybe()

	svc := NewTokenService(manager, store, logger.New(0))

	access, refresh, err := svc.Refresh(ctx, presented)
	require.NoError(t, err)
	assert.Equal(t, "access-new", access)
	assert.Equal(t, "refresh-new", refresh)
}

func TestTokenService_Refresh_Revoked(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	jti := "jti"
	presented := "refresh"
	manager := &servermocks.TokenManager{}
	store := &servermocks.RefreshTokenStore{}

	now := time.Now()

	manager.On("ParseRefreshToken", presented).Return(userID, jti, nil).Once()
	h := sha256.Sum256([]byte(presented))
	hSlice := h[:]
	store.On("GetByJTI", ctx, jti).Return(model.RefreshToken{
		JTI:       jti,
		UserID:    userID,
		TokenHash: hSlice,
		IssuedAt:  now.Add(-time.Hour),
		ExpiresAt: now.Add(time.Hour),
		RevokedAt: &now,
	}, nil).Once()

	svc := NewTokenService(manager, store, logger.New(0))

	_, _, err := svc.Refresh(ctx, presented)
	require.ErrorIs(t, err, model.ErrTokenRevoked)
}

func TestTokenService_Refresh_Expired(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	jti := "jti"
	presented := "refresh"
	manager := &servermocks.TokenManager{}
	store := &servermocks.RefreshTokenStore{}

	manager.On("ParseRefreshToken", presented).Return(userID, jti, nil).Once()
	h2 := sha256.Sum256([]byte(presented))
	h2Slice := h2[:]
	store.On("GetByJTI", ctx, jti).Return(model.RefreshToken{
		JTI:       jti,
		UserID:    userID,
		TokenHash: h2Slice,
		IssuedAt:  time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Minute),
	}, nil).Once()

	svc := NewTokenService(manager, store, logger.New(0))

	_, _, err := svc.Refresh(ctx, presented)
	require.ErrorIs(t, err, model.ErrTokenExpired)
}

func TestTokenService_Refresh_Mismatch(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	jti := "jti"
	presented := "refresh"
	manager := &servermocks.TokenManager{}
	store := &servermocks.RefreshTokenStore{}

	manager.On("ParseRefreshToken", presented).Return(userID, jti, nil).Once()
	h3 := sha256.Sum256([]byte("other"))
	h3Slice := h3[:]
	store.On("GetByJTI", ctx, jti).Return(model.RefreshToken{
		JTI:       jti,
		UserID:    userID,
		TokenHash: h3Slice,
		IssuedAt:  time.Now().Add(-time.Hour),
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil).Once()

	svc := NewTokenService(manager, store, logger.New(0))

	_, _, err := svc.Refresh(ctx, presented)
	require.ErrorIs(t, err, model.ErrTokenMismatch)
}

func TestTokenService_RevokeByToken(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	jti := "jti"
	presented := "refresh"
	manager := &servermocks.TokenManager{}
	store := &servermocks.RefreshTokenStore{}

	manager.On("ParseRefreshToken", presented).Return(userID, jti, nil).Once()
	store.On("RevokeByJTI", ctx, jti).Return(nil).Once()

	svc := NewTokenService(manager, store, logger.New(0))

	require.NoError(t, svc.RevokeByToken(ctx, presented))
}

func TestTokenService_GetUserID(t *testing.T) {
	manager := &servermocks.TokenManager{}
	store := &servermocks.RefreshTokenStore{}

	u := uuid.New()
	manager.On("ParseAccessToken", "access").Return(u, nil).Once()

	svc := NewTokenService(manager, store, logger.New(0))

	got, err := svc.GetUserID(context.Background(), "access")
	require.NoError(t, err)
	assert.Equal(t, u, got)
}

func TestTokenService_RevokeAllForUser(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	manager := &servermocks.TokenManager{}
	store := &servermocks.RefreshTokenStore{}

	store.On("RevokeAllByUser", ctx, userID).Return(nil).Once()

	svc := NewTokenService(manager, store, logger.New(0))

	require.NoError(t, svc.RevokeAllForUser(ctx, userID))
}

func TestTokenService_RevokeAllForUser_Error(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	manager := &servermocks.TokenManager{}
	store := &servermocks.RefreshTokenStore{}

	store.On("RevokeAllByUser", ctx, userID).Return(assert.AnError).Once()

	svc := NewTokenService(manager, store, logger.New(0))

	require.Error(t, svc.RevokeAllForUser(ctx, userID))
}
