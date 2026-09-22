package gologger

import (
	"context"
	"log/slog"

	"github.com/assurrussa/gologger/handlers/slogcontext"
)

func WithValue(ctx context.Context, attr slog.Attr) context.Context {
	return slogcontext.WithValue(ctx, attr)
}
