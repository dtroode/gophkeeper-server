package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dtroode/gophkeeper-server/internal/testutil"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLogging_HandleGRPC(t *testing.T) {
	t.Parallel()

	lg := NewLogging(testutil.MakeNoopLogger())

	tests := []struct {
		name     string
		handler  grpc.UnaryHandler
		wantCode codes.Code
	}{
		{
			name: "success path",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				time.Sleep(10 * time.Millisecond)
				return "ok", nil
			},
			wantCode: codes.OK,
		},
		{
			name: "grpc error propagates",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, status.Error(codes.InvalidArgument, "bad input")
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "non-grpc error becomes Internal",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, errors.New("boom")
			},
			wantCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}
			resp, err := lg.HandleGRPC(context.Background(), struct{}{}, info, tt.handler)

			if tt.wantCode == codes.OK {
				assert.NoError(t, err)
				assert.Equal(t, "ok", resp)
				return
			}

			st, ok := status.FromError(err)
			gotCode := codes.Internal
			if ok {
				gotCode = st.Code()
			}
			assert.Equal(t, tt.wantCode, gotCode)
		})
	}
}
