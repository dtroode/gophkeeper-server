package middleware

import (
	"context"
	"time"

	"github.com/dtroode/gophkeeper-server/internal/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Logging is a unary interceptor that logs gRPC requests and results.
type Logging struct {
	logger *logger.Logger
}

// NewLogging creates a new Logging middleware.
func NewLogging(logger *logger.Logger) *Logging {
	return &Logging{logger: logger}
}

// HandleGRPC logs method name, duration and status for each unary request.
func (l *Logging) HandleGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()

	// Log incoming request
	l.logger.Info("gRPC request started",
		"method", info.FullMethod,
		"start_time", start.Format(time.RFC3339))

	// Call the handler
	resp, err := handler(ctx, req)

	// Calculate duration
	duration := time.Since(start)

	// Determine status
	statusCode := codes.OK
	if err != nil {
		if st, ok := status.FromError(err); ok {
			statusCode = st.Code()
		} else {
			statusCode = codes.Internal
		}
	}

	// Log completed request
	l.logger.Info("gRPC request completed",
		"method", info.FullMethod,
		"duration_ms", duration.Milliseconds(),
		"status", statusCode.String())

	// Log error details if present
	if err != nil {
		l.logger.Error("gRPC request failed",
			"method", info.FullMethod,
			"error", err.Error(),
			"status", statusCode.String())
	}

	return resp, err
}
