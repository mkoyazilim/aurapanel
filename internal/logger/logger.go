// Package logger, merkezi slog yapılandırmasını kurar.
package logger

import (
	"io"
	"log/slog"
	"strings"
)

// New, seviye ve format ayarlarıyla bir slog.Logger döndürür.
// Format: "json" (varsayılan) veya "text".
func New(level, format string, w io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	var h slog.Handler
	if strings.EqualFold(format, "text") {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}
	return slog.New(h)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
