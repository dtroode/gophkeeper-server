package handler

import (
	"context"
	"testing"

	apiErrors "github.com/dtroode/gophkeeper-api/errors"
	authModel "github.com/dtroode/gophkeeper-auth/model"
	authProto "github.com/dtroode/gophkeeper-auth/server/proto"
	"github.com/dtroode/gophkeeper-server/internal/mocks"
	"github.com/dtroode/gophkeeper-server/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type tokenSvcStub struct{}

func (tokenSvcStub) Refresh(ctx context.Context, refreshToken string) (string, string, error) {
	return "acc", "ref", nil
}
func (tokenSvcStub) RevokeByToken(ctx context.Context, refreshToken string) error { return nil }

type tokenSvcErrStub struct{ err error }

func (t tokenSvcErrStub) Refresh(ctx context.Context, refreshToken string) (string, string, error) {
	return "", "", t.err
}
func (t tokenSvcErrStub) RevokeByToken(ctx context.Context, refreshToken string) error { return t.err }

func TestAuth_GetRegParams(t *testing.T) {
	t.Parallel()

	svc := mocks.NewAuthService(t)
	tokens := tokenSvcStub{}
	lg := testutil.MakeNoopLogger()

	svc.On("GetRegParams", mock.Anything, "user").Return(authModel.RegParams{KDFParams: authModel.KDFParams{Time: 1, MemKiB: 2, Par: 1}, SaltRoot: []byte("s"), SessionID: "sid"}, nil)

	h := NewAuth(svc, tokens, lg)
	out, err := h.GetRegParams(context.Background(), &authProto.RegStart{Login: "user"})
	assert.NoError(t, err)
	assert.Equal(t, "sid", out.SessionId)
}

func TestAuth_GetRegParams_Error(t *testing.T) {
	t.Parallel()

	svc := mocks.NewAuthService(t)
	tokens := tokenSvcStub{}
	lg := testutil.MakeNoopLogger()

	svc.On("GetRegParams", mock.Anything, "user").Return(authModel.RegParams{}, apiErrors.NewErrInternalServerError(assert.AnError))

	h := NewAuth(svc, tokens, lg)
	out, err := h.GetRegParams(context.Background(), &authProto.RegStart{Login: "user"})
	assert.Nil(t, out)
	assert.Error(t, err)
}

func TestAuth_GetLoginParams(t *testing.T) {
	t.Parallel()

	svc := mocks.NewAuthService(t)
	tokens := tokenSvcStub{}
	lg := testutil.MakeNoopLogger()

	svc.On("GetLoginParams", mock.Anything, mock.Anything).Return(authModel.LoginParams{KDFParams: authModel.KDFParams{Time: 1, MemKiB: 2, Par: 1}, SaltRoot: []byte("s"), ServerNonce: []byte("n"), SessionID: "sid"}, nil)

	h := NewAuth(svc, tokens, lg)
	out, err := h.GetLoginParams(context.Background(), &authProto.LoginStart{Login: "u", ClientNonce: []byte("c")})
	assert.NoError(t, err)
	assert.Equal(t, "sid", out.SessionId)
}

func TestAuth_CompleteLogin(t *testing.T) {
	t.Parallel()

	svc := mocks.NewAuthService(t)
	tokens := tokenSvcStub{}
	lg := testutil.MakeNoopLogger()

	svc.On("CompleteLogin", mock.Anything, mock.Anything).Return(authModel.SessionResult{ServerSignature: []byte("sig"), AccessToken: "a", RefreshToken: "r"}, nil)

	h := NewAuth(svc, tokens, lg)
	out, err := h.CompleteLogin(context.Background(), &authProto.LoginComplete{Login: "u"})
	assert.NoError(t, err)
	assert.Equal(t, "a", out.AccessToken)
}

func TestAuth_CompleteReg_Error(t *testing.T) {
	t.Parallel()

	svc := mocks.NewAuthService(t)
	tokens := tokenSvcStub{}
	lg := testutil.MakeNoopLogger()

	svc.On("CompleteReg", mock.Anything, mock.Anything).Return(apiErrors.NewErrSignup())

	h := NewAuth(svc, tokens, lg)
	out, err := h.CompleteReg(context.Background(), &authProto.RegComplete{Login: "u", KdfParams: &authProto.KDFParams{Time: 1, MemKib: 2, Par: 1}, SaltRoot: []byte("s"), StoredKey: []byte("a"), ServerKey: []byte("b")})
	assert.Nil(t, out)
	assert.Error(t, err)
}

func TestAuth_CompleteReg_Success(t *testing.T) {
	t.Parallel()

	svc := mocks.NewAuthService(t)
	tokens := tokenSvcStub{}
	lg := testutil.MakeNoopLogger()

	svc.On("CompleteReg", mock.Anything, mock.Anything).Return(nil)

	h := NewAuth(svc, tokens, lg)
	out, err := h.CompleteReg(context.Background(), &authProto.RegComplete{
		Login:     "u",
		KdfParams: &authProto.KDFParams{Time: 1, MemKib: 2, Par: 1},
		SaltRoot:  []byte("s"), StoredKey: []byte("a"), ServerKey: []byte("b"),
	})
	assert.NoError(t, err)
	assert.NotNil(t, out)
}

func TestAuth_GetLoginParams_Error(t *testing.T) {
	t.Parallel()

	svc := mocks.NewAuthService(t)
	tokens := tokenSvcStub{}
	lg := testutil.MakeNoopLogger()

	svc.On("GetLoginParams", mock.Anything, mock.Anything).Return(authModel.LoginParams{}, apiErrors.NewErrLogin())

	h := NewAuth(svc, tokens, lg)
	out, err := h.GetLoginParams(context.Background(), &authProto.LoginStart{})
	assert.Nil(t, out)
	assert.Error(t, err)
}

func TestAuth_CompleteLogin_Error(t *testing.T) {
	t.Parallel()

	svc := mocks.NewAuthService(t)
	tokens := tokenSvcStub{}
	lg := testutil.MakeNoopLogger()

	svc.On("CompleteLogin", mock.Anything, mock.Anything).Return(authModel.SessionResult{}, apiErrors.NewErrLogin())

	h := NewAuth(svc, tokens, lg)
	out, err := h.CompleteLogin(context.Background(), &authProto.LoginComplete{})
	assert.Nil(t, out)
	assert.Error(t, err)
}

func TestAuth_RefreshToken_Error(t *testing.T) {
	t.Parallel()

	svc := mocks.NewAuthService(t)
	tokens := tokenSvcErrStub{err: assert.AnError}
	lg := testutil.MakeNoopLogger()

	h := NewAuth(svc, tokens, lg)
	out, err := h.RefreshToken(context.Background(), &authProto.RefreshTokenRequest{RefreshToken: "x"})
	assert.Nil(t, out)
	assert.Error(t, err)
}

func TestAuth_RevokeToken_Error(t *testing.T) {
	t.Parallel()

	svc := mocks.NewAuthService(t)
	tokens := tokenSvcErrStub{err: assert.AnError}
	lg := testutil.MakeNoopLogger()

	h := NewAuth(svc, tokens, lg)
	out, err := h.RevokeToken(context.Background(), &authProto.RevokeTokenRequest{RefreshToken: "x"})
	assert.Nil(t, out)
	assert.Error(t, err)
}

func TestAuth_RefreshToken_Validation(t *testing.T) {
	t.Parallel()

	svc := mocks.NewAuthService(t)
	tokens := tokenSvcStub{}
	lg := testutil.MakeNoopLogger()

	h := NewAuth(svc, tokens, lg)
	out, err := h.RefreshToken(context.Background(), &authProto.RefreshTokenRequest{RefreshToken: ""})
	assert.Nil(t, out)
	assert.Error(t, err)
}

func TestAuth_RevokeToken_Validation(t *testing.T) {
	t.Parallel()

	svc := mocks.NewAuthService(t)
	tokens := tokenSvcStub{}
	lg := testutil.MakeNoopLogger()

	h := NewAuth(svc, tokens, lg)
	out, err := h.RevokeToken(context.Background(), &authProto.RevokeTokenRequest{RefreshToken: ""})
	assert.Nil(t, out)
	assert.Error(t, err)
}

func TestAuth_RefreshToken_Success(t *testing.T) {
	t.Parallel()

	svc := mocks.NewAuthService(t)
	tokens := tokenSvcStub{}
	lg := testutil.MakeNoopLogger()

	h := NewAuth(svc, tokens, lg)
	out, err := h.RefreshToken(context.Background(), &authProto.RefreshTokenRequest{RefreshToken: "r"})
	assert.NoError(t, err)
	assert.Equal(t, "acc", out.AccessToken)
	assert.Equal(t, "ref", out.RefreshToken)
}

func TestAuth_RevokeToken_Success(t *testing.T) {
	t.Parallel()

	svc := mocks.NewAuthService(t)
	tokens := tokenSvcStub{}
	lg := testutil.MakeNoopLogger()

	h := NewAuth(svc, tokens, lg)
	out, err := h.RevokeToken(context.Background(), &authProto.RevokeTokenRequest{RefreshToken: "r"})
	assert.NoError(t, err)
	assert.NotNil(t, out)
}
