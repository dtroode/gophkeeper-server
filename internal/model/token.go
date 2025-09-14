package model

import "github.com/google/uuid"

// TokenManager generates and validates access/refresh tokens.
type TokenManager interface {
	GenerateAccessToken(userID uuid.UUID) (string, error)
	GenerateRefreshToken(userID uuid.UUID) (token string, jti string, err error)
	ParseAccessToken(token string) (uuid.UUID, error)
	ParseRefreshToken(token string) (userID uuid.UUID, jti string, err error)
}
