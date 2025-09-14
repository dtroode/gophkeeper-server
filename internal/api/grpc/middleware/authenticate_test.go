package middleware

import (
	"context"
	"testing"

	apiErrors "github.com/dtroode/gophkeeper-api/errors"
	"github.com/dtroode/gophkeeper-server/internal/mocks"
	"github.com/dtroode/gophkeeper-server/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestAuthenticate_AuthFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		mdAuthHeader   string
		tokenSvcUserID uuid.UUID
		tokenSvcErr    error
		wantGRPCCode   codes.Code
		wantErr        bool
		expectSetCtx   bool
	}{
		{
			name:         "missing authorization header",
			mdAuthHeader: "",
			tokenSvcErr:  apiErrors.NewErrMissingAuthorizationToken(),
			wantGRPCCode: codes.Unauthenticated,
			wantErr:      true,
			expectSetCtx: false,
		},
		{
			name:         "invalid token",
			mdAuthHeader: "Bearer invalid",
			tokenSvcErr:  apiErrors.NewErrInvalidAuthorizationToken(),
			wantGRPCCode: codes.Unauthenticated,
			wantErr:      true,
			expectSetCtx: false,
		},
		{
			name:           "nil user id from token",
			mdAuthHeader:   "Bearer token",
			tokenSvcUserID: uuid.Nil,
			tokenSvcErr:    nil,
			wantGRPCCode:   codes.Unauthenticated,
			wantErr:        true,
			expectSetCtx:   false,
		},
		{
			name:           "valid token",
			mdAuthHeader:   "Bearer token",
			tokenSvcUserID: uuid.New(),
			tokenSvcErr:    nil,
			wantGRPCCode:   codes.OK,
			wantErr:        false,
			expectSetCtx:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lg := testutil.MakeNoopLogger()
			cm := mocks.NewContextManager(t)

			if tt.expectSetCtx {
				cm.On("SetUserIDToContext", mock.Anything, tt.tokenSvcUserID).Return(context.Background())
			}

			svc := mocks.NewTokenService(t)
			if tt.mdAuthHeader != "" {
				svc.On("GetUserID", mock.Anything, mock.AnythingOfType("string")).Return(tt.tokenSvcUserID, tt.tokenSvcErr)
			}
			m := NewAuthenticate(svc, cm, lg)

			ctx := context.Background()
			if tt.mdAuthHeader != "" {
				ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", tt.mdAuthHeader))
			}

			newCtx, err := m.AuthFunc(ctx)

			if tt.wantErr {
				assert.Error(t, err)
				st, ok := status.FromError(err)
				assert.True(t, ok)
				assert.Equal(t, tt.wantGRPCCode, st.Code())
				assert.Nil(t, newCtx)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, newCtx)
			}
		})
	}
}
