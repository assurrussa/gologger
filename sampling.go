package gologger

import (
	"context"
	"log/slog"
)

// Uniform sampling decides once per record, so all destinations agree. The
// production random function is math/rand/v2.Float64, which is concurrency-safe.
type samplingHandler struct {
	slog.Handler
	rate   float64
	random func() float64
}

func (h *samplingHandler) Handle(ctx context.Context, record slog.Record) error {
	if h.random() >= h.rate {
		return nil
	}
	return h.Handler.Handle(ctx, record)
}

func (h *samplingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &samplingHandler{Handler: h.Handler.WithAttrs(attrs), rate: h.rate, random: h.random}
}

func (h *samplingHandler) WithGroup(name string) slog.Handler {
	return &samplingHandler{Handler: h.Handler.WithGroup(name), rate: h.rate, random: h.random}
}
