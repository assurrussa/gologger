package gologger

import (
	"context"
	"log/slog"
)

// Logger is the consumer-facing logging contract. Implementations may wrap any
// slog.Handler; lifecycle ownership remains with the concrete logger's creator.
type Logger interface {
	Handler() slog.Handler
	DebugContext(ctx context.Context, msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
	LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr)
	WithNamed(name string) Logger
	WithAttrs(attrs ...slog.Attr) Logger
}
