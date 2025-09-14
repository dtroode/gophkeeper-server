package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config contains server configuration parameters.
type Config struct {
	LogLevel int      `env:"LOG_LEVEL" envDefault:"0"`
	GRPC     GRPC     `envPrefix:"GRPC_"`
	Database Database `envPrefix:"DATABASE_"`
	KDF      KDF      `envPrefix:"KDF_"`
	JWT      JWT      `envPrefix:"JWT_"`
	Storage  Storage  `envPrefix:"MINIO_"`
}

// KDF contains KDF parameters for auth protocols.
type KDF struct {
	Time   uint32 `env:"TIME"`
	MemKiB uint32 `env:"MEM"`
	Par    uint8  `env:"PAR"`
}

// GRPC contains gRPC server parameters.
type GRPC struct {
	Port               string `env:"PORT" envDefault:"50051"`
	EnableHTTPS        bool   `env:"ENABLE_HTTPS" envDefault:"false"`
	CertFileName       string `env:"CERT_FILE_NAME" envDefault:"cert.pem"`
	PrivateKeyFileName string `env:"PRIVATE_KEY_FILE_NAME" envDefault:"key.pem"`
}

// Database contains database connection parameters.
type Database struct {
	DSN string `env:"DSN" envDefault:"postgres://gophkeeper:gophkeeper@localhost:5432/gophkeeper?sslmode=disable"`
}

// JWT contains JWT-related parameters.
type JWT struct {
	Secret string `env:"SECRET" envDefault:"devsecret"`
}

// Storage contains object storage parameters.
type Storage struct {
	Endpoint  string `env:"ENDPOINT" envDefault:"localhost:9000"`
	AccessKey string `env:"ACCESS_KEY" envDefault:"gophkeeper-access-key"`
	SecretKey string `env:"SECRET_KEY" envDefault:"gophkeeper-secret-key"`
	Bucket    string `env:"BUCKET_NAME" envDefault:"gophkeeper-files"`
	UseSSL    bool   `env:"USE_SSL" envDefault:"false"`
}

// NewConfig loads configuration from environment variables.
func NewConfig() (*Config, error) {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}
