package server

import (
	"crypto/tls"
	"fmt"
	"net"
)

// TLSListener represents a TLS-enabled network listener.
// It provides secure network connections using TLS certificates.
type TLSListener struct {
	certFileName       string
	privateKeyFileName string
}

// NewTLSListener creates a new TLSListener instance.
// It initializes a TLS listener with the specified certificate and private key files.
//
// Parameters:
//   - certFileName: Path to the TLS certificate file
//   - privateKeyFileName: Path to the private key file
//
// Returns a pointer to the newly created TLSListener instance.
func NewTLSListener(certFileName, privateKeyFileName string) *TLSListener {
	return &TLSListener{
		certFileName:       certFileName,
		privateKeyFileName: privateKeyFileName,
	}
}

// Listen creates a TLS-enabled network listener.
// It loads the TLS certificate and private key, then creates a secure listener.
//
// Parameters:
//   - protocol: The network protocol (typically "tcp")
//   - addr: The address to listen on
//
// Returns a TLS-enabled network listener or an error if setup fails.
func (l *TLSListener) Listen(protocol, addr string) (net.Listener, error) {
	cert, err := tls.LoadX509KeyPair(l.certFileName, l.privateKeyFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	return tls.Listen("tcp", addr, tlsConfig)
}

// PlainListener represents a plain (non-TLS) network listener.
// It provides unencrypted network connections.
type PlainListener struct{}

// NewPlainListener creates a new PlainListener instance.
// It initializes a plain network listener without TLS encryption.
//
// Returns a pointer to the newly created PlainListener instance.
func NewPlainListener() *PlainListener {
	return &PlainListener{}
}

// Listen creates a plain network listener.
// It creates an unencrypted TCP listener on the specified address.
//
// Parameters:
//   - protocol: The network protocol (typically "tcp")
//   - addr: The address to listen on
//
// Returns a plain network listener or an error if setup fails.
func (l *PlainListener) Listen(protocol, addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}
