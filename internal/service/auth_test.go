package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	authmodel "github.com/dtroode/gophkeeper-auth/model"
	"github.com/dtroode/gophkeeper-server/internal/logger"
	servermocks "github.com/dtroode/gophkeeper-server/internal/mocks"
	"github.com/dtroode/gophkeeper-server/internal/model"
)

type fakeProtocol struct{}

func (f *fakeProtocol) PrepareRegistration(ctx context.Context) (authmodel.RegParams, error) {
	return authmodel.RegParams{SessionID: uuid.NewString(), KDFParams: authmodel.KDFParams{Time: 1, MemKiB: 1024, Par: 1}, SaltRoot: []byte("salt")}, nil
}
func (f *fakeProtocol) VerifyRegistration(ctx context.Context, _ authmodel.PendingReg, _ authmodel.RegComplete) error {
	return nil
}
func (f *fakeProtocol) PrepareLogin(ctx context.Context) (authmodel.LoginParams, error) {
	return authmodel.LoginParams{SessionID: uuid.NewString(), ServerNonce: []byte{1, 2, 3}}, nil
}
func (f *fakeProtocol) VerifyLogin(ctx context.Context, _ []byte, _ authmodel.PendingLogin, _ authmodel.LoginComplete) error {
	return nil
}
func (f *fakeProtocol) MakeServerSignature(login string, serverKey, clientNonce, serverNonce []byte) []byte {
	return []byte{9}
}

func TestAuth_GetRegParams_NewUser(t *testing.T) {
	ctx := context.Background()
	userStore := &servermocks.UserStore{}
	signupStore := &servermocks.SignupStore{}
	loginStore := &servermocks.LoginStore{}
	refreshStore := &servermocks.RefreshTokenStore{}
	tokMan := &servermocks.TokenManager{}
	log := logger.New(0)

	userStore.On("GetByEmail", mock.Anything, "a@b.c").Return(model.User{}, model.ErrNotFound)
	signupStore.On("Create", mock.Anything, mock.Anything).Return(nil)

	a := NewAuth(userStore, signupStore, loginStore, refreshStore, log, tokMan, authmodel.KDFParams{Time: 1, MemKiB: 1024, Par: 1})
	// override protocol
	a.protocol = &fakeProtocol{}

	params, err := a.GetRegParams(ctx, "a@b.c")
	require.NoError(t, err)
	assert.NotEmpty(t, params.SessionID)
	assert.NotEmpty(t, params.SaltRoot)
}

func TestAuth_GetRegParams_ExistingUser(t *testing.T) {
	ctx := context.Background()
	userStore := &servermocks.UserStore{}
	signupStore := &servermocks.SignupStore{}
	loginStore := &servermocks.LoginStore{}
	refreshStore := &servermocks.RefreshTokenStore{}
	tokMan := &servermocks.TokenManager{}
	log := logger.New(0)

	userStore.On("GetByEmail", mock.Anything, "existing@user.com").Return(model.User{ID: uuid.New()}, nil)

	a := NewAuth(userStore, signupStore, loginStore, refreshStore, log, tokMan, authmodel.KDFParams{Time: 1, MemKiB: 1024, Par: 1})
	a.protocol = &fakeProtocol{}

	_, err := a.GetRegParams(ctx, "existing@user.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already taken")
}

func TestAuth_CompleteReg_Success(t *testing.T) {
	ctx := context.Background()
	userStore := &servermocks.UserStore{}
	signupStore := &servermocks.SignupStore{}
	loginStore := &servermocks.LoginStore{}
	refreshStore := &servermocks.RefreshTokenStore{}
	tokMan := &servermocks.TokenManager{}
	log := logger.New(0)

	pending := model.PendingSignup{SessionID: "sid", Login: "a@b.c", SaltRoot: []byte("salt"), KDF: mustJSON(t, authmodel.KDFParams{Time: 1})}
	signupStore.On("GetBySessionID", mock.Anything, "sid").Return(pending, nil)
	userStore.On("GetByEmail", mock.Anything, "a@b.c").Return(model.User{}, model.ErrNotFound)
	signupStore.On("Consume", mock.Anything, "sid").Return(nil)
	userStore.On("Create", mock.Anything, mock.Anything).Return(model.User{ID: uuid.New()}, nil)

	a := NewAuth(userStore, signupStore, loginStore, refreshStore, log, tokMan, authmodel.KDFParams{Time: 1})
	a.protocol = &fakeProtocol{}

	require.NoError(t, a.CompleteReg(ctx, authmodel.RegComplete{SessionID: "sid", Login: "a@b.c", SaltRoot: []byte("salt"), KDF: authmodel.KDFParams{Time: 1}, StoredKey: make([]byte, 32), ServerKey: make([]byte, 32)}))
}

func TestAuth_GetLoginParams_UserNotFound(t *testing.T) {
	ctx := context.Background()
	userStore := &servermocks.UserStore{}
	loginStore := &servermocks.LoginStore{}
	signupStore := &servermocks.SignupStore{}
	refreshStore := &servermocks.RefreshTokenStore{}
	tokMan := &servermocks.TokenManager{}

	userStore.On("GetByEmail", mock.Anything, "x").Return(model.User{}, model.ErrNotFound)

	a := NewAuth(userStore, signupStore, loginStore, refreshStore, logger.New(0), tokMan, authmodel.KDFParams{Time: 1})
	a.protocol = &fakeProtocol{}

	_, err := a.GetLoginParams(ctx, authmodel.LoginStart{Login: "x", ClientNonce: []byte{1}})
	require.Error(t, err)
}

func TestAuth_CompleteLogin_Success(t *testing.T) {
	ctx := context.Background()
	userStore := &servermocks.UserStore{}
	loginStore := &servermocks.LoginStore{}
	signupStore := &servermocks.SignupStore{}
	refreshStore := &servermocks.RefreshTokenStore{}
	tokMan := &servermocks.TokenManager{}
	log := logger.New(0)

	user := model.User{ID: uuid.New(), Email: "a", StoredKey: make([]byte, 32), ServerKey: make([]byte, 32), KDF: mustJSON(t, authmodel.KDFParams{Time: 1})}
	userStore.On("GetByEmail", mock.Anything, "a").Return(user, nil)
	loginStore.On("GetBySessionID", mock.Anything, "sid").Return(model.PendingLogin{SessionID: "sid", Login: "a", ClientNonce: []byte{1}, ServerNonce: []byte{2}, ExpiresAt: time.Now().Add(time.Hour)}, nil)
	loginStore.On("Consume", mock.Anything, "sid").Return(nil)
	tokMan.On("GenerateAccessToken", user.ID).Return("at", nil)
	tokMan.On("GenerateRefreshToken", user.ID).Return("rt", "jti", nil)
	refreshStore.On("Create", mock.Anything, mock.Anything).Return(nil)

	a := NewAuth(userStore, signupStore, loginStore, refreshStore, log, tokMan, authmodel.KDFParams{Time: 1})
	a.protocol = &fakeProtocol{}

	res, err := a.CompleteLogin(ctx, authmodel.LoginComplete{SessionID: "sid", Login: "a", ClientNonce: []byte{1}, ServerNonce: []byte{2}})
	require.NoError(t, err)
	assert.Equal(t, "at", res.AccessToken)
	assert.Equal(t, "rt", res.RefreshToken)
}

func TestAuth_GetLoginParams_Success(t *testing.T) {
	ctx := context.Background()
	userStore := &servermocks.UserStore{}
	signupStore := &servermocks.SignupStore{}
	loginStore := &servermocks.LoginStore{}
	refreshStore := &servermocks.RefreshTokenStore{}
	tokMan := &servermocks.TokenManager{}
	log := logger.New(0)

	userID := uuid.New()
	user := model.User{ID: userID, Email: "test@user.com", KDF: mustJSON(t, authmodel.KDFParams{Time: 1, MemKiB: 1024, Par: 1})}

	userStore.On("GetByEmail", mock.Anything, "test@user.com").Return(user, nil)
	loginStore.On("Create", mock.Anything, mock.Anything).Return(nil)

	a := NewAuth(userStore, signupStore, loginStore, refreshStore, log, tokMan, authmodel.KDFParams{Time: 1, MemKiB: 1024, Par: 1})
	a.protocol = &fakeProtocol{}

	params, err := a.GetLoginParams(ctx, authmodel.LoginStart{Login: "test@user.com", ClientNonce: []byte("nonce")})
	require.NoError(t, err)
	assert.NotEmpty(t, params.SessionID)
	assert.NotEmpty(t, params.ServerNonce)
}

func mustJSON(t *testing.T, v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestAuth_CompleteReg_GetPendingError(t *testing.T) {
	ctx := context.Background()
	userStore := &servermocks.UserStore{}
	signupStore := &servermocks.SignupStore{}
	loginStore := &servermocks.LoginStore{}
	refreshStore := &servermocks.RefreshTokenStore{}
	tokMan := &servermocks.TokenManager{}
	log := logger.New(0)

	signupStore.On("GetBySessionID", mock.Anything, "session-id").Return(model.PendingSignup{}, assert.AnError)

	a := NewAuth(userStore, signupStore, loginStore, refreshStore, log, tokMan, authmodel.KDFParams{Time: 1, MemKiB: 1024, Par: 1})
	a.protocol = &fakeProtocol{}

	err := a.CompleteReg(ctx, authmodel.RegComplete{SessionID: "session-id", Login: "test@user.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get pending signup")
}
