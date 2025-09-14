package logger

import (
	"log/slog"
	"os"
)

// Logger represents application logger.
type Logger struct {
	*slog.Logger
}

// New creates new Logger instance with the specified level.
func New(level int) *Logger {
	return &Logger{
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.Level(level)})),
	}
}

// Fatal is equivalent to Error followed by os.Exit(1).
func (l *Logger) Fatal(msg string, args ...any) {
	l.Logger.Error(msg, args...)
	os.Exit(1)
}
