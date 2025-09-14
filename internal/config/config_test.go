package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig_DefaultValues(t *testing.T) {
	cfg, err := NewConfig()
	require.NoError(t, err)

	assert.Equal(t, 0, cfg.LogLevel)
	assert.Equal(t, "50051", cfg.GRPC.Port)
	assert.Equal(t, false, cfg.GRPC.EnableHTTPS)
	assert.Equal(t, "cert.pem", cfg.GRPC.CertFileName)
	assert.Equal(t, "key.pem", cfg.GRPC.PrivateKeyFileName)
	assert.Equal(t, "postgres://gophkeeper:gophkeeper@localhost:5432/gophkeeper?sslmode=disable", cfg.Database.DSN)
	assert.Equal(t, "devsecret", cfg.JWT.Secret)
	assert.Equal(t, "localhost:9000", cfg.Storage.Endpoint)
	assert.Equal(t, "gophkeeper-access-key", cfg.Storage.AccessKey)
	assert.Equal(t, "gophkeeper-secret-key", cfg.Storage.SecretKey)
	assert.Equal(t, "gophkeeper-files", cfg.Storage.Bucket)
	assert.Equal(t, false, cfg.Storage.UseSSL)
}

func TestNewConfig_EnvironmentOverrides(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected func(*Config)
	}{
		{
			name: "log level override",
			envVars: map[string]string{
				"LOG_LEVEL": "2",
			},
			expected: func(cfg *Config) {
				assert.Equal(t, 2, cfg.LogLevel)
			},
		},
		{
			name: "grpc config override",
			envVars: map[string]string{
				"GRPC_PORT":                  "8080",
				"GRPC_ENABLE_HTTPS":          "true",
				"GRPC_CERT_FILE_NAME":        "custom.pem",
				"GRPC_PRIVATE_KEY_FILE_NAME": "custom-key.pem",
			},
			expected: func(cfg *Config) {
				assert.Equal(t, "8080", cfg.GRPC.Port)
				assert.Equal(t, true, cfg.GRPC.EnableHTTPS)
				assert.Equal(t, "custom.pem", cfg.GRPC.CertFileName)
				assert.Equal(t, "custom-key.pem", cfg.GRPC.PrivateKeyFileName)
			},
		},
		{
			name: "database config override",
			envVars: map[string]string{
				"DATABASE_DSN": "postgres://user:pass@host:5432/db",
			},
			expected: func(cfg *Config) {
				assert.Equal(t, "postgres://user:pass@host:5432/db", cfg.Database.DSN)
			},
		},
		{
			name: "kdf config override",
			envVars: map[string]string{
				"KDF_TIME": "3",
				"KDF_MEM":  "128000",
				"KDF_PAR":  "4",
			},
			expected: func(cfg *Config) {
				assert.Equal(t, uint32(3), cfg.KDF.Time)
				assert.Equal(t, uint32(128000), cfg.KDF.MemKiB)
				assert.Equal(t, uint8(4), cfg.KDF.Par)
			},
		},
		{
			name: "jwt config override",
			envVars: map[string]string{
				"JWT_SECRET": "customsecret",
			},
			expected: func(cfg *Config) {
				assert.Equal(t, "customsecret", cfg.JWT.Secret)
			},
		},
		{
			name: "storage config override",
			envVars: map[string]string{
				"MINIO_ENDPOINT":    "minio.example.com:9000",
				"MINIO_ACCESS_KEY":  "access123",
				"MINIO_SECRET_KEY":  "secret123",
				"MINIO_BUCKET_NAME": "custom-bucket",
				"MINIO_USE_SSL":     "true",
			},
			expected: func(cfg *Config) {
				assert.Equal(t, "minio.example.com:9000", cfg.Storage.Endpoint)
				assert.Equal(t, "access123", cfg.Storage.AccessKey)
				assert.Equal(t, "secret123", cfg.Storage.SecretKey)
				assert.Equal(t, "custom-bucket", cfg.Storage.Bucket)
				assert.Equal(t, true, cfg.Storage.UseSSL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.envVars {
				os.Setenv(key, value)
				defer os.Unsetenv(key)
			}

			cfg, err := NewConfig()
			require.NoError(t, err)

			tt.expected(cfg)
		})
	}
}
