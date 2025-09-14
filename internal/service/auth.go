package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	apiErrors "github.com/dtroode/gophkeeper-api/errors"
	"github.com/dtroode/gophkeeper-server/internal/logger"
	"github.com/dtroode/gophkeeper-server/internal/model"
	"github.com/google/uuid"

	auth "github.com/dtroode/gophkeeper-auth/model"
	scram "github.com/dtroode/gophkeeper-auth/scram"
)

type Auth struct {
	userStore    model.UserStore
	signupStore  model.SignupStore
	loginStore   model.LoginStore
	protocol     auth.ServerAuth
	tokenService *TokenService
	logger       *logger.Logger
}

func NewAuth(
	userStore model.UserStore,
	signupStore model.SignupStore,
	loginStore model.LoginStore,
	refreshTokenStore model.RefreshTokenStore,
	logger *logger.Logger,
	tokenManager model.TokenManager,
	kdf auth.KDFParams,
) *Auth {
	return &Auth{
		userStore:    userStore,
		signupStore:  signupStore,
		loginStore:   loginStore,
		tokenService: NewTokenService(tokenManager, refreshTokenStore, logger),
		logger:       logger,
		protocol:     scram.NewBaseServerProtocol(kdf, logger),
	}
}

func (a *Auth) GetRegParams(ctx context.Context, login string) (auth.RegParams, error) {
	a.logger.Debug("Auth service: starting user registration",
		"login", login)

	existingUser, err := a.userStore.GetByEmail(ctx, login)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		a.logger.Error("Auth service: failed to get user by email",
			"login", login,
			"error", err.Error())
		return auth.RegParams{}, fmt.Errorf("failed to get user by email: %w", err)
	}

	if existingUser.ID != uuid.Nil {
		a.logger.Info("Auth service: user already exists",
			"login", login)
		return auth.RegParams{}, apiErrors.NewErrEmailIsTaken(login)
	}

	regParams, err := a.protocol.PrepareRegistration(ctx)
	if err != nil {
		a.logger.Error("Auth service: failed to get registration params",
			"login", login,
			"error", err.Error())
		return auth.RegParams{}, fmt.Errorf("failed to get server params: %w", err)
	}

	marshaledKDF, err := json.Marshal(regParams.KDFParams)
	if err != nil {
		a.logger.Error("Auth service: failed to marshal KDF params",
			"login", login,
			"error", err.Error())
		return auth.RegParams{}, fmt.Errorf("failed to marshal kdf params: %w", err)
	}

	pendingSignup := model.PendingSignup{
		SessionID: regParams.SessionID,
		Login:     login,
		SaltRoot:  regParams.SaltRoot,
		KDF:       marshaledKDF,
		ExpiresAt: time.Now().Add(model.PendingSessionDuration),
	}

	err = a.signupStore.Create(ctx, pendingSignup)
	if err != nil {
		a.logger.Error("Auth service: failed to create pending signup",
			"login", login,
			"session_id", regParams.SessionID,
			"error", err.Error())
		return auth.RegParams{}, fmt.Errorf("failed to create pending signup: %w", err)
	}

	a.logger.Info("Auth service: registration started successfully",
		"login", login,
		"session_id", regParams.SessionID)

	return regParams, nil
}

func (a *Auth) CompleteReg(ctx context.Context, params auth.RegComplete) error {
	a.logger.Debug("Auth service: finishing user registration",
		"login", params.Login,
		"session_id", params.SessionID)

	pendingSignup, err := a.signupStore.GetBySessionID(ctx, params.SessionID)
	if err != nil {
		a.logger.Error("Auth service: failed to get pending signup",
			"session_id", params.SessionID,
			"error", err.Error())
		return fmt.Errorf("failed to get pending signup by session id: %w", err)
	}

	err = a.protocol.VerifyRegistration(ctx,
		auth.PendingReg{
			SessionID: pendingSignup.SessionID,
			Login:     pendingSignup.Login,
			SaltRoot:  pendingSignup.SaltRoot,
			KDF:       pendingSignup.KDF,
			ExpiresAt: pendingSignup.ExpiresAt,
			Consumed:  pendingSignup.Consumed,
		},
		params)
	if err != nil {
		return err
	}

	userWithLogin, err := a.userStore.GetByEmail(ctx, params.Login)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("failed to get user by email: %w", err)
	}

	if userWithLogin.ID != uuid.Nil {
		return apiErrors.NewErrEmailIsTaken(params.Login)
	}

	// todo: add transaction
	err = a.signupStore.Consume(ctx, pendingSignup.SessionID)
	if err != nil {
		return fmt.Errorf("failed to consume signup session: %w", err)
	}

	user := model.User{
		ID:        uuid.New(),
		Email:     params.Login,
		StoredKey: params.StoredKey,
		ServerKey: params.ServerKey,
		SaltRoot:  params.SaltRoot,
		KDF:       pendingSignup.KDF,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = a.userStore.Create(ctx, user)
	if err != nil {
		a.logger.Error("Auth service: failed to create user",
			"login", params.Login,
			"error", err.Error())
		return fmt.Errorf("failed to create user: %w", err)
	}

	a.logger.Info("Auth service: user registration completed successfully",
		"login", params.Login,
		"session_id", params.SessionID)

	return nil
}

func (a *Auth) GetLoginParams(ctx context.Context, params auth.LoginStart) (auth.LoginParams, error) {
	a.logger.Debug("Auth service: starting user login",
		"login", params.Login)

	user, err := a.userStore.GetByEmail(ctx, params.Login)
	if errors.Is(err, model.ErrNotFound) {
		return auth.LoginParams{}, apiErrors.NewErrUserNotFound(params.Login)
	}
	if err != nil {
		return auth.LoginParams{}, fmt.Errorf("failed to get user by email: %w", err)
	}

	sessionParams, err := a.protocol.PrepareLogin(ctx)
	if err != nil {
		return auth.LoginParams{}, fmt.Errorf("failed to get server login params: %w", err)
	}

	pendingLogin := model.PendingLogin{
		SessionID:   sessionParams.SessionID,
		Login:       params.Login,
		ClientNonce: params.ClientNonce,
		ServerNonce: sessionParams.ServerNonce,
		ExpiresAt:   time.Now().Add(model.PendingSessionDuration),
		Consumed:    false,
	}

	err = a.loginStore.Create(ctx, pendingLogin)
	if err != nil {
		return auth.LoginParams{}, fmt.Errorf("failed to create pending login: %w", err)
	}

	a.logger.Info("Auth service: login started successfully",
		"login", params.Login,
		"session_id", sessionParams.SessionID)

	var kdf auth.KDFParams
	err = json.Unmarshal(user.KDF, &kdf)
	if err != nil {
		return auth.LoginParams{}, fmt.Errorf("failed to unmarshal user kdf: %w", err)
	}

	return auth.LoginParams{
		SessionID:   sessionParams.SessionID,
		ServerNonce: sessionParams.ServerNonce,
		SaltRoot:    user.SaltRoot,
		KDFParams: auth.KDFParams{
			Time:   kdf.Time,
			MemKiB: kdf.MemKiB,
			Par:    kdf.Par,
		},
	}, nil
}

func (a *Auth) CompleteLogin(ctx context.Context, params auth.LoginComplete) (auth.SessionResult, error) {
	a.logger.Debug("Auth service: finishing user login",
		"login", params.Login,
		"session_id", params.SessionID)

	user, err := a.userStore.GetByEmail(ctx, params.Login)
	if err != nil {
		return auth.SessionResult{}, fmt.Errorf("failed to get user by email: %w", err)
	}

	pendingLogin, err := a.loginStore.GetBySessionID(ctx, params.SessionID)
	if err != nil {
		return auth.SessionResult{}, fmt.Errorf("failed to get pending login: %w", err)
	}

	err = a.protocol.VerifyLogin(ctx, user.StoredKey, auth.PendingLogin{
		SessionID:   pendingLogin.SessionID,
		Login:       pendingLogin.Login,
		ClientNonce: pendingLogin.ClientNonce,
		ServerNonce: pendingLogin.ServerNonce,
		ExpiresAt:   pendingLogin.ExpiresAt,
		Consumed:    pendingLogin.Consumed,
	}, params)
	if err != nil {
		return auth.SessionResult{}, fmt.Errorf("failed to verify login: %w", err)
	}

	serverSignature := a.protocol.MakeServerSignature(params.Login, user.ServerKey, pendingLogin.ClientNonce, pendingLogin.ServerNonce)

	err = a.loginStore.Consume(ctx, pendingLogin.SessionID)
	if err != nil {
		return auth.SessionResult{}, fmt.Errorf("failed to consume login session: %w", err)
	}

	accessToken, refreshToken, err := a.tokenService.Issue(ctx, user.ID)
	if err != nil {
		return auth.SessionResult{}, fmt.Errorf("failed to issue token: %w", err)
	}

	return auth.SessionResult{
		ServerSignature: serverSignature,
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
	}, nil
}
