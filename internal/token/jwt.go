package token

import (
	"fmt"
	"time"

	"github.com/dtroode/gophkeeper-server/internal/model"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims represents JWT claims with token type and user ID.
type Claims struct {
	jwt.RegisteredClaims
	UserID    uuid.UUID `json:"user_id"`
	TokenType string    `json:"typ"`
}

// JWT implements TokenManager backed by symmetric HMAC.
type JWT struct {
	secretKey string
}

// NewJWT creates a new JWT token manager with the provided secret key.
func NewJWT(secretKey string) model.TokenManager {
	return &JWT{secretKey: secretKey}
}

const (
	accessTTL   = 15 * time.Minute
	refreshTTL  = 30 * 24 * time.Hour
	typeAccess  = "access"
	typeRefresh = "refresh"
)

// GenerateAccessToken creates a short-lived access token.
func (j *JWT) GenerateAccessToken(userID uuid.UUID) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTTL)),
		},
		UserID:    userID,
		TokenType: typeAccess,
	})

	tokenString, err := token.SignedString([]byte(j.secretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign access token: %w", err)
	}

	return tokenString, nil
}

// GenerateRefreshToken creates a long-lived refresh token and returns its JTI.
func (j *JWT) GenerateRefreshToken(userID uuid.UUID) (string, string, error) {
	now := time.Now()
	jti := uuid.NewString()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(refreshTTL)),
		},
		UserID:    userID,
		TokenType: typeRefresh,
	})

	tokenString, err := token.SignedString([]byte(j.secretKey))
	if err != nil {
		return "", "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return tokenString, jti, nil
}

// ParseAccessToken validates and extracts the user ID from an access token.
func (j *JWT) ParseAccessToken(tokenString string) (uuid.UUID, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("wrong signing method %v", t.Header["alg"])
		}
		return []byte(j.secretKey), nil
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to parse access token: %w", err)
	}
	if !token.Valid {
		return uuid.Nil, fmt.Errorf("access token is invalid")
	}
	if claims.TokenType != typeAccess {
		return uuid.Nil, fmt.Errorf("token type mismatch: %s", claims.TokenType)
	}
	return claims.UserID, nil
}

// ParseRefreshToken validates and extracts the user ID and JTI from a refresh token.
func (j *JWT) ParseRefreshToken(tokenString string) (uuid.UUID, string, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("wrong signing method %v", t.Header["alg"])
		}
		return []byte(j.secretKey), nil
	})
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("failed to parse refresh token: %w", err)
	}
	if !token.Valid {
		return uuid.Nil, "", fmt.Errorf("refresh token is invalid")
	}
	if claims.TokenType != typeRefresh {
		return uuid.Nil, "", fmt.Errorf("token type mismatch: %s", claims.TokenType)
	}
	return claims.UserID, claims.ID, nil
}
