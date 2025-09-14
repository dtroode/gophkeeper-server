package middleware

import (
	"context"
	"strings"

	apiErrors "github.com/dtroode/gophkeeper-api/errors"
	"github.com/dtroode/gophkeeper-server/internal/logger"
	"github.com/dtroode/gophkeeper-server/internal/model"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TokenService resolves user ID from bearer tokens.
type TokenService interface {
	GetUserID(ctx context.Context, token string) (uuid.UUID, error)
}

// Authenticate validates bearer tokens and injects user ID into context.
type Authenticate struct {
	tokenService   TokenService
	contextManager model.ContextManager
	logger         *logger.Logger
}

// NewAuthenticate creates a new Authenticate middleware instance.
func NewAuthenticate(tokenService TokenService, contextManager model.ContextManager, logger *logger.Logger) *Authenticate {
	return &Authenticate{tokenService: tokenService, contextManager: contextManager, logger: logger}
}

// AuthFunc parses Authorization header, validates token and returns a context with user ID.
func (m *Authenticate) AuthFunc(ctx context.Context) (context.Context, error) {
	var tokenString string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if authHeaders := md.Get("authorization"); len(authHeaders) > 0 {
			tokenString = strings.TrimPrefix(authHeaders[0], "Bearer ")
		}
	}

	userID, authErr := m.authenticateUser(ctx, tokenString)
	if authErr != nil {
		return nil, status.Error(codes.Unauthenticated, authErr.Error())
	}

	return m.contextManager.SetUserIDToContext(ctx, userID), nil
}

func (m *Authenticate) authenticateUser(ctx context.Context, tokenString string) (userID uuid.UUID, err error) {
	if tokenString == "" {
		return uuid.Nil, apiErrors.NewErrMissingAuthorizationToken()
	}

	userID, err = m.tokenService.GetUserID(ctx, tokenString)
	if err != nil {
		return uuid.Nil, apiErrors.NewErrInvalidAuthorizationToken()
	}

	if userID == uuid.Nil {
		return uuid.Nil, apiErrors.NewErrInvalidAuthorizationToken()
	}

	return userID, nil
}
