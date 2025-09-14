package testutil

import (
	"io"
	"log/slog"

	"github.com/dtroode/gophkeeper-server/internal/logger"
)

func MakeNoopLogger() *logger.Logger {
	return &logger.Logger{Logger: slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))}
}
