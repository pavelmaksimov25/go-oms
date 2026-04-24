package logger

import (
	"io"
	"log/slog"
	"os"
)

// New builds a JSON slog.Logger with the service name attached as a base attr.
// Useful for tests; Init wraps it to also install the logger as slog.Default.
func New(serviceName string, w io.Writer) *slog.Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(handler).With(slog.String("service", serviceName))
}

// Init installs a JSON logger with a service=<name> base attr as slog.Default
// and returns it for callers that want to pass it around explicitly.
func Init(serviceName string) *slog.Logger {
	l := New(serviceName, os.Stdout)
	slog.SetDefault(l)
	return l
}
