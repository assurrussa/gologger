package slogcontext

import (
	"context"
	"log/slog"
)

var _ slog.Handler = (*Handler)(nil)

type ctxKey string

const (
	slogFields ctxKey = "slog_fields"
)

type Handler struct {
	slog.Handler
}

func NewHandler(handler slog.Handler) slog.Handler {
	return &Handler{Handler: handler}
}

func (h Handler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.Handler.Enabled(ctx, l)
}

// Handle adds contextual attributes to the Record before calling the underlying
// handler.
func (h Handler) Handle(ctx context.Context, r slog.Record) error {
	if attrs, ok := ctx.Value(slogFields).([]slog.Attr); ok {
		r = r.Clone()
		for _, v := range attrs {
			r.AddAttrs(v)
		}
	}

	return h.Handler.Handle(ctx, r)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{Handler: h.Handler.WithGroup(name)}
}

// WithValue adds an slog attribute to the provided context so that it will be
// included in any Record created with such context.
func WithValue(parent context.Context, attr slog.Attr) context.Context {
	if parent == nil {
		parent = context.Background()
	}

	if v, ok := parent.Value(slogFields).([]slog.Attr); ok {
		v = append(v[:len(v):len(v)], attr)
		return context.WithValue(parent, slogFields, v)
	}

	v := make([]slog.Attr, 0, 1)
	v = append(v, attr)

	return context.WithValue(parent, slogFields, v)
}
