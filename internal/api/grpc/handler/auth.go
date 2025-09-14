package handler

import (
	"context"

	apiErrors "github.com/dtroode/gophkeeper-api/errors"
	"github.com/dtroode/gophkeeper-auth/server/proto"
	"github.com/dtroode/gophkeeper-server/internal/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/dtroode/gophkeeper-auth/model"
)

// AuthService defines user registration and login operations.
type AuthService interface {
	GetRegParams(ctx context.Context, login string) (model.RegParams, error)
	CompleteReg(ctx context.Context, params model.RegComplete) error
	GetLoginParams(ctx context.Context, params model.LoginStart) (model.LoginParams, error)
	CompleteLogin(ctx context.Context, params model.LoginComplete) (model.SessionResult, error)
}

// TokenService defines token refresh and revoke operations.
type TokenService interface {
	Refresh(ctx context.Context, refreshToken string) (accessToken string, newRefreshToken string, err error)
	RevokeByToken(ctx context.Context, refreshToken string) error
}

// Auth handles gRPC endpoints for authentication.
type Auth struct {
	proto.UnimplementedAuthServer
	authService  AuthService
	tokenService TokenService
	logger       *logger.Logger
}

// NewAuth creates a new Auth handler.
func NewAuth(authService AuthService, tokenService TokenService, logger *logger.Logger) *Auth {
	return &Auth{
		authService:  authService,
		tokenService: tokenService,
		logger:       logger,
	}
}

// GetRegParams starts registration and returns server KDF and salt parameters.
func (h *Auth) GetRegParams(ctx context.Context, req *proto.RegStart) (*proto.RegParams, error) {
	h.logger.Debug("Auth handler: processing registration start request",
		"login", req.Login)

	response, err := h.authService.GetRegParams(ctx, req.Login)
	if err != nil {
		h.logger.Error("Auth handler: registration start failed",
			"login", req.Login,
			"error", err.Error())
		return nil, h.handleError(err)
	}

	h.logger.Info("Auth handler: registration start completed",
		"login", req.Login,
		"session_id", response.SessionID)

	return &proto.RegParams{
		KdfParams: &proto.KDFParams{
			Time:   response.KDFParams.Time,
			MemKib: response.KDFParams.MemKiB,
			Par:    uint32(response.KDFParams.Par),
		},
		SaltRoot:  response.SaltRoot,
		SessionId: response.SessionID,
	}, nil
}

// CompleteReg finishes registration with verifiers.
func (h *Auth) CompleteReg(ctx context.Context, req *proto.RegComplete) (*emptypb.Empty, error) {
	h.logger.Debug("Auth handler: processing registration finish request",
		"login", req.Login,
		"session_id", req.SessionId)

	params := model.RegComplete{
		SessionID: req.SessionId,
		Login:     req.Login,
		SaltRoot:  req.SaltRoot,
		KDF: model.KDFParams{
			Time:   req.KdfParams.Time,
			MemKiB: req.KdfParams.MemKib,
			Par:    uint8(req.KdfParams.Par),
		},
		StoredKey: req.StoredKey,
		ServerKey: req.ServerKey,
	}

	err := h.authService.CompleteReg(ctx, params)
	if err != nil {
		h.logger.Error("Auth handler: registration finish failed",
			"login", req.Login,
			"session_id", req.SessionId,
			"error", err.Error())
		return nil, h.handleError(err)
	}

	h.logger.Info("Auth handler: registration finish completed",
		"login", req.Login,
		"session_id", req.SessionId)

	return &emptypb.Empty{}, nil
}

// GetLoginParams starts login and returns server nonce and KDF params.
func (h *Auth) GetLoginParams(ctx context.Context, req *proto.LoginStart) (*proto.LoginParams, error) {
	h.logger.Debug("Auth handler: processing login start request",
		"login", req.Login)

	response, err := h.authService.GetLoginParams(ctx, model.LoginStart{
		Login:       req.Login,
		ClientNonce: req.ClientNonce,
	})
	if err != nil {
		h.logger.Error("Auth handler: login start failed",
			"login", req.Login,
			"error", err.Error())
		return nil, h.handleError(err)
	}

	h.logger.Info("Auth handler: login start completed",
		"login", req.Login,
		"server_nonce", response.ServerNonce)

	return &proto.LoginParams{
		KdfParams: &proto.KDFParams{
			Time:   response.KDFParams.Time,
			MemKib: response.KDFParams.MemKiB,
			Par:    uint32(response.KDFParams.Par),
		},
		SaltRoot:    response.SaltRoot,
		ServerNonce: response.ServerNonce,
		SessionId:   response.SessionID,
	}, nil
}

// CompleteLogin verifies client proof and returns session tokens.
func (h *Auth) CompleteLogin(ctx context.Context, req *proto.LoginComplete) (*proto.SessionResult, error) {
	h.logger.Debug("Auth handler: processing login finish request",
		"login", req.Login)

	params := model.LoginComplete{
		SessionID:   req.SessionId,
		Login:       req.Login,
		ClientNonce: req.ClientNonce,
		ServerNonce: req.ServerNonce,
		ClientProof: req.ClientProof,
	}

	result, err := h.authService.CompleteLogin(ctx, params)
	if err != nil {
		h.logger.Error("Auth handler: login finish failed",
			"login", req.Login,
			"error", err.Error())
		return nil, h.handleError(err)
	}

	h.logger.Info("Auth handler: login finish completed",
		"login", req.Login,
		"server_signature", result.ServerSignature)

	return &proto.SessionResult{
		ServerSignature: result.ServerSignature,
		AccessToken:     result.AccessToken,
		RefreshToken:    result.RefreshToken,
	}, nil
}

// RefreshToken exchanges refresh token for a new access token (and optional refresh).
func (h *Auth) RefreshToken(ctx context.Context, req *proto.RefreshTokenRequest) (*proto.RefreshTokenResponse, error) {
	h.logger.Debug("Auth handler: processing token refresh request")

	if req.RefreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh token is required")
	}

	accessToken, refreshToken, err := h.tokenService.Refresh(ctx, req.RefreshToken)
	if err != nil {
		h.logger.Error("Auth handler: token refresh failed",
			"error", err.Error())
		return nil, h.handleError(err)
	}

	h.logger.Info("Auth handler: token refresh successful")

	return &proto.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// RevokeToken revokes a refresh token.
func (h *Auth) RevokeToken(ctx context.Context, req *proto.RevokeTokenRequest) (*emptypb.Empty, error) {
	h.logger.Debug("Auth handler: processing token revoke request")

	if req.RefreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh token is required")
	}

	err := h.tokenService.RevokeByToken(ctx, req.RefreshToken)
	if err != nil {
		h.logger.Error("Auth handler: token revoke failed",
			"error", err.Error())
		return nil, h.handleError(err)
	}

	h.logger.Info("Auth handler: token revoke successful")

	return &emptypb.Empty{}, nil
}

func (h *Auth) handleError(err error) error {
	apiErr, ok := err.(*apiErrors.APIError)
	if !ok {
		apiErr = apiErrors.NewErrInternalServerError(err)
	}

	return status.Errorf(apiErr.GRPCCode, apiErr.Template, apiErr.Args)
}
