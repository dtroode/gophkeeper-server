package model

import "errors"

var (
	ErrTokenRevoked  = errors.New("refresh token revoked")
	ErrTokenExpired  = errors.New("refresh token expired")
	ErrTokenMismatch = errors.New("refresh token mismatch")
)
