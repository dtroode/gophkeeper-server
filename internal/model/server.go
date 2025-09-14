package model

import (
	"context"
	"net"
)

type SecurityLayer interface {
	Listen(protocol, addr string) (net.Listener, error)
}

type Server interface {
	Start(securityLayer SecurityLayer) error
	Stop(ctx context.Context) error
	Address() string
}
