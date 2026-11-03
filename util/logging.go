package util

import (
	"log/slog"
	"os"
)

// SetupLogging makes a JSON logger that writes to stdout the default
// logger. Call this first in main, anything logged before this goes to
// stderr in plain text.
//
// NOTE: This also redirects the standard log package to this logger.
func SetupLogging() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))
}
