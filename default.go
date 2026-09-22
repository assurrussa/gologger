package gologger

import (
	"io"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/assurrussa/gologger/handlers/slogcontext"
)

// LogLevel controls legacy NewLogger and Discard loggers. Use WithLevel for new
// code. Mutate it through Set; replacing this pointer is unsupported.
var LogLevel = legacyLevel()

var defaultLogger atomic.Pointer[Log]

var defaultMu sync.Mutex

func legacyLevel() *slog.LevelVar {
	level := new(slog.LevelVar)
	level.Set(slog.LevelWarn)
	return level
}

// Default returns this package's default without changing slog.Default.
func Default() *Log {
	if log := defaultLogger.Load(); log != nil {
		return log
	}
	log, err := New(Config{}, WithLevel(LogLevel))
	if err != nil {
		panic(err) // All options above are fixed and valid.
	}
	defaultLogger.CompareAndSwap(nil, log)
	return defaultLogger.Load()
}

// SetDefault explicitly installs log as both the package and slog default.
// The host remains responsible for closing any previous logger. log must be non-nil.
func SetDefault(log *Log) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	slog.SetDefault(log.Logger)
	defaultLogger.Store(log)
}

// NewLogger configures the global logger with a shared level, WARN fallback and
// local pretty output. Prefer New for independent instances.
// Use New followed by SetDefault only when process wiring requires it.
func NewLogger(cfg Config, handlers ...OptionHandler) (*Log, error) {
	if err := LogLevel.UnmarshalText([]byte(cfg.Level)); err != nil {
		LogLevel.Set(slog.LevelWarn)
	}
	if !(cfg.Rate > 0 && cfg.Rate < 1) {
		cfg.Rate = 0
	}
	// The original global constructor ignored JSON in local environments.
	cfg.JSON = false
	log, err := New(cfg, WithLevel(LogLevel), WithAdditionalHandlers(handlers...))
	if err != nil {
		return nil, err
	}
	SetDefault(log)
	return log, nil
}

// Discard returns a no-op logger with optional additional destinations.
// Additional handlers receive the legacy shared LogLevel.
func Discard(handlers ...OptionHandler) *Log {
	destinations := []slog.Handler{slog.DiscardHandler}
	for _, factory := range handlers {
		if h := factory(&slog.HandlerOptions{Level: LogLevel}); h != nil {
			destinations = append(destinations, h)
		}
	}
	return &Log{Logger: slog.New(slogcontext.NewHandler(slog.NewMultiHandler(destinations...)))}
}

func DiscardJSONWithWriter(writer io.Writer) *Log {
	return Discard(func(opts *slog.HandlerOptions) slog.Handler { return slog.NewJSONHandler(writer, opts) })
}

func DiscardTextWithWriter(writer io.Writer) *Log {
	return Discard(func(opts *slog.HandlerOptions) slog.Handler { return slog.NewTextHandler(writer, opts) })
}
