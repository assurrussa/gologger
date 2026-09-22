// Package gologger provides configurable slog loggers, context attributes and
// public handler extension points. New never changes the process default logger.
package gologger

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"os"
	"runtime"
	"runtime/debug"
	"slices"
	"sync"

	"github.com/assurrussa/gologger/handlers/slogcontext"
	"github.com/assurrussa/gologger/handlers/slogpretty"
)

var _ Logger = (*Log)(nil)

// Log embeds slog.Logger and shares resource ownership with WithNamed/WithAttrs
// children. Close the root after all users have stopped logging.
type Log struct {
	*slog.Logger
	lifecycle *lifecycle
}

type lifecycle struct {
	once    sync.Once
	closers []io.Closer
	writer  io.Writer
	err     error
}

// New builds an independent logger. The default is JSON on stdout at WARN;
// local/development uses pretty output unless Config.JSON is true.
// It starts no goroutines and does not change LogLevel or slog.Default.
func New(cfg Config, opts ...Option) (*Log, error) {
	o := options{writer: os.Stdout}
	for _, option := range opts {
		if option == nil {
			return nil, errors.New("nil logger option")
		}
		option(&o)
	}
	if err := o.validate(cfg); err != nil {
		return nil, err
	}
	handlerOpts := slog.HandlerOptions{Level: o.level, AddSource: cfg.AddSource, ReplaceAttr: o.replaceAttr}
	primary := o.handler
	if !o.custom {
		primary = slog.NewJSONHandler(o.writer, &handlerOpts)
		if cfg.IsLocal() && !cfg.JSON {
			primary = slogpretty.NewHandler(o.writer, &slogpretty.HandlerOptions{SlogOpts: &handlerOpts})
		}
	}
	handlers := []slog.Handler{primary}
	for _, factory := range o.handlers {
		factoryOpts := handlerOpts
		if h := factory(&factoryOpts); h != nil {
			handlers = append(handlers, h)
		}
	}
	var sink *fileSink
	if cfg.Output != "" {
		file, err := os.OpenFile(cfg.Output, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
		if err != nil {
			return nil, fmt.Errorf("create file %q: %w", cfg.Output, err)
		}
		sink = newFileSink(file, 1024*1024)
		handlers = append(handlers, slog.NewJSONHandler(sink, &handlerOpts))
	}
	var handler slog.Handler = slog.NewMultiHandler(handlers...)
	if cfg.Rate > 0 && cfg.Rate < 1 {
		handler = &samplingHandler{Handler: handler, rate: cfg.Rate, random: rand.Float64}
	}
	for _, mw := range slices.Backward(o.middleware) {
		handler = mw(handler)
		if handler == nil {
			var closeErr error
			if sink != nil {
				closeErr = sink.Close()
			}
			return nil, errors.Join(errors.New("middleware returned nil handler"), closeErr)
		}
	}
	log := slog.New(slogcontext.NewHandler(handler))
	if cfg.AddVerbose {
		version := ""
		if info, ok := debug.ReadBuildInfo(); ok {
			version = info.Main.Version
		}
		log = log.With(slog.Group("program_info",
			slog.Int("pid", os.Getpid()), slog.String("version", version), slog.String("go_version", runtime.Version())))
	}
	resources := &lifecycle{writer: o.writer}
	if sink != nil {
		resources.closers = append(resources.closers, sink)
	}
	resources.closers = append(resources.closers, o.closers...)
	return &Log{Logger: log, lifecycle: resources}, nil
}

func (o *options) validate(cfg Config) error {
	if o.writer == nil {
		return errors.New("nil log writer")
	}
	if o.custom && o.handler == nil {
		return errors.New("nil log handler")
	}
	if math.IsNaN(cfg.Rate) || cfg.Rate < 0 || cfg.Rate > 1 {
		return errors.New("sampling rate must be between 0 and 1")
	}
	if o.level == nil {
		level := new(slog.LevelVar)
		level.Set(slog.LevelWarn)
		if cfg.Level != "" {
			if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
				return fmt.Errorf("parse log level: %w", err)
			}
		}
		o.level = level
	}
	for _, factory := range o.handlers {
		if factory == nil {
			return errors.New("nil additional handler factory")
		}
	}
	for _, middleware := range o.middleware {
		if middleware == nil {
			return errors.New("nil logger middleware")
		}
	}
	for _, closer := range o.closers {
		if closer == nil {
			return errors.New("nil logger closer")
		}
	}
	return nil
}

func (l *Log) WithNamed(name string) Logger {
	return &Log{Logger: l.With(slog.String("name", name)), lifecycle: l.lifecycle}
}

func (l *Log) WithAttrs(attrs ...slog.Attr) Logger {
	return &Log{Logger: slog.New(l.Handler().WithAttrs(attrs)), lifecycle: l.lifecycle}
}

// Close flushes and closes owned resources once. Every caller receives the same
// result, including concurrent calls through derived loggers. Borrowed writers
// and handlers are never closed. Logging after Close is unsupported.
func (l *Log) Close() error {
	if l == nil || l.lifecycle == nil {
		return nil
	}
	resources := l.lifecycle
	resources.once.Do(func() {
		for _, closer := range slices.Backward(resources.closers) {
			resources.err = errors.Join(resources.err, closer.Close())
		}
		if f, ok := resources.writer.(*os.File); ok {
			if info, err := f.Stat(); err == nil && info.Mode().IsRegular() {
				resources.err = errors.Join(resources.err, f.Sync())
			}
		}
	})
	return resources.err
}

// Flush is a compatibility name for Close. It is terminal, not a periodic flush.
func (l *Log) Flush() error { return l.Close() }
