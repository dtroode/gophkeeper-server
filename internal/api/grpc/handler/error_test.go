package handler

import (
	"errors"
	"testing"

	apiErrors "github.com/dtroode/gophkeeper-api/errors"
	"github.com/dtroode/gophkeeper-server/internal/model"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestHandleError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       error
		wantCode codes.Code
		wantMsg  string
	}{
		{
			name:     "api error passthrough",
			in:       apiErrors.NewErrInvalidRecordType("bin"),
			wantCode: codes.InvalidArgument,
			wantMsg:  apiErrors.NewErrInvalidRecordType("bin").Message,
		},
		{
			name:     "model not found -> NotFound",
			in:       model.ErrNotFound,
			wantCode: codes.NotFound,
			wantMsg:  "record not found",
		},
		{
			name:     "other -> Internal",
			in:       errors.New("boom"),
			wantCode: codes.Internal,
			wantMsg:  "internal server error",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := handleError(tt.in)
			st, ok := status.FromError(err)
			assert.True(t, ok)
			assert.Equal(t, tt.wantCode, st.Code())
			assert.Equal(t, tt.wantMsg, st.Message())
		})
	}
}
