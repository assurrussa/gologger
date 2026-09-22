package gologger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/assurrussa/gologger"
)

func TestIndependentLoggers(t *testing.T) {
	before, level := slog.Default(), gologger.LogLevel.Level()
	var first, second bytes.Buffer
	debugLog, err := gologger.New(gologger.Config{Level: "debug"}, gologger.WithWriter(&first))
	if err != nil {
		t.Fatal(err)
	}
	warnLog, err := gologger.New(gologger.Config{Level: "warn"}, gologger.WithWriter(&second))
	if err != nil {
		t.Fatal(err)
	}
	debugLog.Info("visible")
	warnLog.Info("hidden")
	if !strings.Contains(first.String(), "visible") || second.Len() != 0 {
		t.Fatalf("independent levels lost: %q / %q", first.String(), second.String())
	}
	if slog.Default() != before || gologger.LogLevel.Level() != level {
		t.Fatal("New changed process defaults")
	}
}

func TestDynamicLevelAndFactoryIsolation(t *testing.T) {
	var first, second bytes.Buffer
	level := new(slog.LevelVar)
	level.Set(slog.LevelError)
	log, err := gologger.New(gologger.Config{Level: "invalid overridden value"},
		gologger.WithWriter(&first), gologger.WithLevel(level),
		gologger.WithAdditionalHandlers(func(opts *slog.HandlerOptions) slog.Handler {
			opts.Level = slog.LevelDebug
			return nil
		}, func(opts *slog.HandlerOptions) slog.Handler { return slog.NewJSONHandler(&second, opts) }))
	if err != nil {
		t.Fatal(err)
	}
	log.Info("hidden")
	if first.Len() != 0 || second.Len() != 0 {
		t.Fatal("handler factory mutated shared options")
	}
	level.Set(slog.LevelDebug)
	log.Debug("visible")
	if !strings.Contains(first.String(), "visible") || !strings.Contains(second.String(), "visible") {
		t.Fatal("runtime level change was not shared")
	}
}

func TestExtensionPipeline(t *testing.T) {
	var output, extra bytes.Buffer
	var order []string
	wrap := func(name string) gologger.Middleware {
		return func(next slog.Handler) slog.Handler {
			return &observer{Handler: next, handle: func(record slog.Record) {
				order = append(order, name)
				found := false
				record.Attrs(func(attr slog.Attr) bool { found = found || attr.Key == "request_id"; return true })
				if !found {
					t.Error("middleware did not see context attributes")
				}
			}}
		}
	}
	log, err := gologger.New(gologger.Config{},
		gologger.WithHandler(slog.NewJSONHandler(&output, nil)),
		gologger.WithAdditionalHandlers(func(opts *slog.HandlerOptions) slog.Handler { return slog.NewJSONHandler(&extra, opts) }),
		gologger.WithMiddleware(wrap("outer"), wrap("inner")))
	if err != nil {
		t.Fatal(err)
	}
	ctx := gologger.WithValue(context.Background(), slog.String("request_id", "req-1"))
	log.WithNamed("worker").WithAttrs(slog.String("tenant", "a")).WarnContext(ctx, "event")
	if !reflect.DeepEqual(order, []string{"outer", "inner"}) {
		t.Fatalf("middleware order: %v", order)
	}
	for _, buffer := range []*bytes.Buffer{&output, &extra} {
		var record map[string]any
		if err := json.Unmarshal(buffer.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		if record["request_id"] != "req-1" || record["name"] != "worker" || record["tenant"] != "a" {
			t.Fatalf("lost derived attributes: %v", record)
		}
	}
}

type observer struct {
	slog.Handler
	handle func(slog.Record)
}

func (h *observer) Handle(ctx context.Context, record slog.Record) error {
	h.handle(record)
	return h.Handler.Handle(ctx, record)
}

func (h *observer) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &observer{Handler: h.Handler.WithAttrs(attrs), handle: h.handle}
}

func (h *observer) WithGroup(name string) slog.Handler {
	return &observer{Handler: h.Handler.WithGroup(name), handle: h.handle}
}

func TestJSONOverrideAndReplaceAttr(t *testing.T) {
	var output bytes.Buffer
	log, err := gologger.New(gologger.Config{Env: "local", JSON: true, AddSource: true, AddVerbose: true},
		gologger.WithWriter(&output),
		gologger.WithReplaceAttr(func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == "password" {
				return slog.String(attr.Key, "[redacted]")
			}
			return attr
		}))
	if err != nil {
		t.Fatal(err)
	}
	log.Warn("hello", "password", "secret")
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("JSON override ignored: %v", err)
	}
	if record["password"] != "[redacted]" || record["program_info"] == nil || record["source"] == nil {
		t.Fatalf("configured attributes missing: %v", record)
	}
	source, ok := record["source"].(map[string]any)
	if !ok || !strings.HasSuffix(source["file"].(string), "logger_test.go") {
		t.Fatalf("source points at wrapper: %v", record["source"])
	}
}

func TestFileOutputAndSharedShutdown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application.log")
	log, err := gologger.New(gologger.Config{Output: path, Level: "info"}, gologger.WithWriter(io.Discard))
	if err != nil {
		t.Fatal(err)
	}
	child := log.WithNamed("child")
	var wg sync.WaitGroup
	for range 30 {
		wg.Go(func() { child.InfoContext(context.Background(), "persist") })
	}
	wg.Wait()
	for range 20 {
		wg.Go(func() {
			if err := child.(*gologger.Log).Flush(); err != nil {
				t.Error(err)
			}
		})
		wg.Go(func() {
			if err := log.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(contents, []byte("\n")) != 30 {
		t.Fatalf("missing records: %s", contents)
	}
}

type trackedCloser struct {
	bytes.Buffer
	count atomic.Int32
	err   error
}

func (c *trackedCloser) Close() error { c.count.Add(1); return c.err }

func TestOwnedAndBorrowedResources(t *testing.T) {
	failure := errors.New("close failed")
	owned := &trackedCloser{err: failure}
	borrowed := new(trackedCloser)
	log, err := gologger.New(gologger.Config{}, gologger.WithWriter(borrowed), gologger.WithClosers(owned))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if err := log.Close(); !errors.Is(err, failure) {
				t.Errorf("lost close failure: %v", err)
			}
		})
	}
	wg.Wait()
	if owned.count.Load() != 1 || borrowed.count.Load() != 0 {
		t.Fatal("resource ownership violated")
	}
	_, err = gologger.New(gologger.Config{Output: filepath.Join(t.TempDir(), "missing", "log")}, gologger.WithClosers(borrowed))
	if err == nil || borrowed.count.Load() != 0 {
		t.Fatal("failed construction took caller resource ownership")
	}
}

func TestInvalidConfiguration(t *testing.T) {
	for _, cfg := range []gologger.Config{{Level: "typo"}, {Rate: -1}, {Rate: 2}, {Rate: math.NaN()}, {Rate: math.Inf(1)}} {
		if log, err := gologger.New(cfg); err == nil || log != nil {
			t.Errorf("accepted invalid config: %+v", cfg)
		}
	}
	for _, option := range []gologger.Option{nil, gologger.WithWriter(nil), gologger.WithHandler(nil), gologger.WithAdditionalHandlers(nil), gologger.WithMiddleware(nil), gologger.WithClosers(nil), gologger.WithMiddleware(func(slog.Handler) slog.Handler { return nil })} {
		if log, err := gologger.New(gologger.Config{}, option); err == nil || log != nil {
			t.Error("accepted invalid option")
		}
	}
}

func TestLegacyDefaultAndDiscard(t *testing.T) {
	previous, previousSlog, previousLevel := gologger.Default(), slog.Default(), gologger.LogLevel.Level()
	t.Cleanup(func() {
		gologger.SetDefault(previous)
		slog.SetDefault(previousSlog)
		gologger.LogLevel.Set(previousLevel)
	})
	var output bytes.Buffer
	log, err := gologger.NewLogger(gologger.Config{Level: "debug"}, func(opts *slog.HandlerOptions) slog.Handler { return slog.NewJSONHandler(&output, opts) })
	if err != nil {
		t.Fatal(err)
	}
	if gologger.Default() != log || slog.Default() != log.Logger || gologger.LogLevel.Level() != slog.LevelDebug {
		t.Fatal("legacy default behavior changed")
	}
	gologger.DiscardJSONWithWriter(&output).Debug("legacy")
	if !strings.Contains(output.String(), "legacy") {
		t.Fatal("discard writer lost legacy level")
	}
	if gologger.Discard().Enabled(context.Background(), slog.LevelError) {
		t.Fatal("discard enabled logging")
	}
}

func TestConcurrentDefaultInstallation(t *testing.T) {
	previous, previousSlog := gologger.Default(), slog.Default()
	t.Cleanup(func() { gologger.SetDefault(previous); slog.SetDefault(previousSlog) })
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			log, err := gologger.New(gologger.Config{}, gologger.WithHandler(slog.DiscardHandler))
			if err != nil {
				t.Error(err)
				return
			}
			for range 20 {
				gologger.SetDefault(log)
			}
		})
	}
	wg.Wait()
	if gologger.Default().Logger != slog.Default() {
		t.Fatal("package and slog defaults diverged")
	}
}
