package router

import (
	"context"
	"strings"

	"github.com/dtroode/gophkeeper-api/proto"
	authProto "github.com/dtroode/gophkeeper-auth/server/proto"
	"github.com/dtroode/gophkeeper-server/internal/api/grpc/handler"
	"github.com/dtroode/gophkeeper-server/internal/api/grpc/middleware"
	"github.com/dtroode/gophkeeper-server/internal/logger"
	"github.com/dtroode/gophkeeper-server/internal/model"
	"github.com/dtroode/gophkeeper-server/internal/service"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/auth"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/selector"
	"google.golang.org/grpc"
)

// Router represents a gRPC router for gophkeeper operations.
// It manages gRPC service registration and middleware configuration.
type Router struct {
	authService    *service.Auth
	recordService  *service.Record
	tokenService   *service.TokenService
	logger         *logger.Logger
	contextManager model.ContextManager
}

// New creates new gRPC Router instance.
// It initializes a gRPC router with auth and record services.
//
// Parameters:
//   - authService: The authentication service
//   - recordService: The record management service
//   - logger: The logger for request logging
//
// Returns a pointer to the newly created Router instance.
func New(
	authService *service.Auth,
	recordService *service.Record,
	tokenService *service.TokenService,
	contextManager model.ContextManager,
	logger *logger.Logger,
) *Router {
	return &Router{
		authService:    authService,
		recordService:  recordService,
		tokenService:   tokenService,
		contextManager: contextManager,
		logger:         logger,
	}
}

func authSkip(_ context.Context, c interceptors.CallMeta) bool {
	return !strings.HasPrefix(c.FullMethod(), "/api.Auth/")
}

// Register registers all gRPC services and middleware.
// It sets up the gRPC server with request logging and authentication interceptors.
//
// Returns the configured gRPC server instance.
func (r *Router) Register() *grpc.Server {
	logging := middleware.NewLogging(r.logger)
	authenticate := middleware.NewAuthenticate(r.tokenService, r.contextManager, r.logger)

	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			logging.HandleGRPC,
			selector.UnaryServerInterceptor(
				auth.UnaryServerInterceptor(authenticate.AuthFunc),
				selector.MatchFunc(authSkip),
			),
		),
		grpc.ChainStreamInterceptor(
			selector.StreamServerInterceptor(
				auth.StreamServerInterceptor(authenticate.AuthFunc),
				selector.MatchFunc(authSkip),
			),
		),
	)
	r.registerAuthRoutes(s)
	r.registerRecordRoutes(s)

	return s
}

func (r *Router) registerAuthRoutes(server *grpc.Server) {
	authHandler := handler.NewAuth(r.authService, r.tokenService, r.logger)
	authProto.RegisterAuthServer(server, authHandler)
}

func (r *Router) registerRecordRoutes(server *grpc.Server) {
	recordHandler := handler.NewRecord(r.recordService, r.contextManager, r.logger)
	proto.RegisterAPIServer(server, recordHandler)
}
