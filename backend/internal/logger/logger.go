package logger

import (
	"io"
	"log/slog"
	"strings"
)

// New constructs a slog.Logger with a JSON handler writing to w.
// level must be one of "debug", "info", "warn", "error" (case-insensitive).
// An unrecognised level defaults to info and does not error.
func New(w io.Writer, level string) *slog.Logger {
	var l slog.Level
	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: l}))
}
