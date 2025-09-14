package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestCertificate(t *testing.T, certFile, keyFile string) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	certOut, err := os.Create(certFile)
	require.NoError(t, err)
	defer certOut.Close()

	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	require.NoError(t, err)

	keyOut, err := os.Create(keyFile)
	require.NoError(t, err)
	defer keyOut.Close()

	privKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)

	err = pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privKeyBytes})
	require.NoError(t, err)
}

func TestNewTLSListener(t *testing.T) {
	certFile := "test.crt"
	keyFile := "test.key"

	listener := NewTLSListener(certFile, keyFile)
	require.NotNil(t, listener)
	assert.Equal(t, certFile, listener.certFileName)
	assert.Equal(t, keyFile, listener.privateKeyFileName)
}

func TestTLSListener_Listen_Success(t *testing.T) {
	tempDir := t.TempDir()
	certFile := filepath.Join(tempDir, "test.crt")
	keyFile := filepath.Join(tempDir, "test.key")

	createTestCertificate(t, certFile, keyFile)

	listener := NewTLSListener(certFile, keyFile)

	ln, err := listener.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NotNil(t, ln)
	defer ln.Close()

	_, ok := ln.(net.Listener)
	require.True(t, ok)
}

func TestTLSListener_Listen_InvalidCertificate(t *testing.T) {
	listener := NewTLSListener("nonexistent.crt", "nonexistent.key")

	_, err := listener.Listen("tcp", "127.0.0.1:0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load TLS certificate")
}

func TestTLSListener_Listen_InvalidAddress(t *testing.T) {
	tempDir := t.TempDir()
	certFile := filepath.Join(tempDir, "test.crt")
	keyFile := filepath.Join(tempDir, "test.key")

	createTestCertificate(t, certFile, keyFile)

	listener := NewTLSListener(certFile, keyFile)

	_, err := listener.Listen("tcp", "invalid-address")
	require.Error(t, err)
}

func TestNewPlainListener(t *testing.T) {
	listener := NewPlainListener()
	require.NotNil(t, listener)
}

func TestPlainListener_Listen_Success(t *testing.T) {
	listener := NewPlainListener()

	ln, err := listener.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NotNil(t, ln)
	defer ln.Close()

	tcpListener, ok := ln.(*net.TCPListener)
	require.True(t, ok)
	assert.NotNil(t, tcpListener)
}

func TestPlainListener_Listen_InvalidAddress(t *testing.T) {
	listener := NewPlainListener()

	_, err := listener.Listen("tcp", "invalid-address")
	require.Error(t, err)
}
