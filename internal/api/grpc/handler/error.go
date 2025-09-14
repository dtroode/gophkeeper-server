package handler

import (
	apiErrors "github.com/dtroode/gophkeeper-api/errors"
	"github.com/dtroode/gophkeeper-server/internal/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func handleError(err error) error {
	if apiErr, ok := err.(*apiErrors.APIError); ok {
		return status.Error(apiErr.GRPCCode, apiErr.Message)
	}

	switch err {
	case model.ErrNotFound:
		return status.Error(codes.NotFound, "record not found")
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
