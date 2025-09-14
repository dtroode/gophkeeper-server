package server

import (
	"context"
	"fmt"

	"github.com/dtroode/gophkeeper-server/internal/model"
	"google.golang.org/grpc"
)

// GRPCServer wraps a gRPC server with address and lifecycle methods.
type GRPCServer struct {
	server *grpc.Server
	addr   string
}

// NewGRPCServer creates a GRPCServer with given server and address.
func NewGRPCServer(
	server *grpc.Server,
	addr string,
) *GRPCServer {
	return &GRPCServer{server: server, addr: addr}
}

// Start starts serving on the configured address using the provided security layer.
func (s *GRPCServer) Start(securityLayer model.SecurityLayer) error {
	listener, err := securityLayer.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	return s.server.Serve(listener)
}

// Stop gracefully stops the server.
func (s *GRPCServer) Stop(_ context.Context) error {
	s.server.GracefulStop()
	return nil
}

// Address returns the configured listen address.
func (s *GRPCServer) Address() string {
	return s.addr
}
