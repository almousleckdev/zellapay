// Package logging builds the structured logger used by all binaries.
package logging

import (
	"log/slog"
	"os"
)

// New returns a JSON logger tagged with the service name.
func New(service, level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l})
	return slog.New(h).With("service", service)
}
