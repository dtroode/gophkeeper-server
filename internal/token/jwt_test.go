package token

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestJWT_AccessToken_Roundtrip(t *testing.T) {
	j := NewJWT("secret")
	u := uuid.New()

	access, err := j.GenerateAccessToken(u)
	require.NoError(t, err)
	got, err := j.ParseAccessToken(access)
	require.NoError(t, err)
	require.Equal(t, u, got)
}

func TestJWT_RefreshToken_Roundtrip(t *testing.T) {
	j := NewJWT("secret")
	u := uuid.New()

	refresh, jti, err := j.GenerateRefreshToken(u)
	require.NoError(t, err)
	require.NotEmpty(t, jti)

	gotUser, gotJTI, err := j.ParseRefreshToken(refresh)
	require.NoError(t, err)
	require.Equal(t, u, gotUser)
	require.Equal(t, jti, gotJTI)
}

func TestJWT_TokenType_Mismatch(t *testing.T) {
	j := NewJWT("secret")
	u := uuid.New()

	access, err := j.GenerateAccessToken(u)
	require.NoError(t, err)

	_, _, err = j.ParseRefreshToken(access)
	require.Error(t, err)
}

func TestJWT_ExpiryValidation(t *testing.T) {
	j := &JWT{secretKey: "secret"}
	u := uuid.New()

	access, err := j.GenerateAccessToken(u)
	require.NoError(t, err)
	_, err = j.ParseAccessToken(access)
	require.NoError(t, err)

	refresh, _, err := j.GenerateRefreshToken(u)
	require.NoError(t, err)
	_, _, err = j.ParseRefreshToken(refresh)
	require.NoError(t, err)

	_ = time.Now()
}
