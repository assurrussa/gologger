package gologger

import (
	"io"
	"log/slog"
)

// OptionHandler builds an additional handler using the configured slog options.
// A nil result skips this handler. Each factory receives its own options copy.
type OptionHandler func(*slog.HandlerOptions) slog.Handler

// Middleware wraps a handler. It must preserve slog's concurrency and derivation contracts.
type Middleware func(slog.Handler) slog.Handler

// Option configures a new logger without changing process defaults.
type Option func(*options)

type options struct {
	writer      io.Writer
	handler     slog.Handler
	custom      bool
	level       slog.Leveler
	replaceAttr func([]string, slog.Attr) slog.Attr
	handlers    []OptionHandler
	middleware  []Middleware
	closers     []io.Closer
}

// WithWriter replaces stdout for the built-in handler. The caller owns the writer
// and must flush/close it, or explicitly transfer ownership through WithClosers.
func WithWriter(writer io.Writer) Option {
	return func(o *options) { o.writer = writer }
}

// WithHandler replaces the built-in console handler. The supplied handler owns
// its own level, source and attribute configuration. File output remains additive.
func WithHandler(handler slog.Handler) Option {
	return func(o *options) { o.handler, o.custom = handler, true }
}

// WithLevel overrides Config.Level without mutating the supplied level.
// A *slog.LevelVar permits runtime changes shared only by explicitly wired loggers.
func WithLevel(level slog.Leveler) Option {
	return func(o *options) { o.level = level }
}

// WithReplaceAttr configures built-in handlers and additional handler factories.
func WithReplaceAttr(replace func([]string, slog.Attr) slog.Attr) Option {
	return func(o *options) { o.replaceAttr = replace }
}

// WithAdditionalHandlers adds destinations alongside the primary handler.
func WithAdditionalHandlers(handlers ...OptionHandler) Option {
	copyHandlers := append([]OptionHandler(nil), handlers...)
	return func(o *options) { o.handlers = append(o.handlers, copyHandlers...) }
}

// WithMiddleware wraps the combined destinations after context enrichment and
// before sampling. Middleware executes in the supplied order, first outermost.
func WithMiddleware(middleware ...Middleware) Option {
	copyMiddleware := append([]Middleware(nil), middleware...)
	return func(o *options) { o.middleware = append(o.middleware, copyMiddleware...) }
}

// WithClosers transfers cleanup responsibility on successful construction only.
// Close/Flush calls each closer once in reverse order; a closer must flush its
// own buffers. Derived loggers share the same cleanup state.
func WithClosers(closers ...io.Closer) Option {
	copyClosers := append([]io.Closer(nil), closers...)
	return func(o *options) { o.closers = append(o.closers, copyClosers...) }
}
