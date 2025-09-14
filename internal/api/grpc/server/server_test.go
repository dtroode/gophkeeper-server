package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/dtroode/gophkeeper-server/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
)

func TestGRPCServer_Address(t *testing.T) {
	s := NewGRPCServer(grpc.NewServer(), ":0")
	assert.Equal(t, ":0", s.Address())
}

func TestGRPCServer_Stop(t *testing.T) {
	s := NewGRPCServer(grpc.NewServer(), ":0")
	err := s.Stop(context.Background())
	assert.NoError(t, err)
}

func TestGRPCServer_Start_ListensAndServes(t *testing.T) {
	t.Parallel()

	gs := grpc.NewServer()
	srv := NewGRPCServer(gs, ":0")
	sec := mocks.NewSecurityLayer(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	sec.On("Listen", "tcp", ":0").Return(ln, nil).Run(func(args mock.Arguments) { close(done) })

	go func() { _ = srv.Start(sec) }()
	<-done
	time.Sleep(10 * time.Millisecond)
	_ = srv.Stop(context.Background())
}

type nopListener struct{ net.Conn }

func (n *nopListener) Accept() (net.Conn, error) { return n.Conn, nil }
func (n *nopListener) Close() error              { return nil }
func (n *nopListener) Addr() net.Addr            { return &net.IPAddr{} }
