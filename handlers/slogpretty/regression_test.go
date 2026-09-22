package slogpretty_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/assurrussa/gologger/handlers/slogpretty"
)

func TestPrettyPreservesDuplicateFields(t *testing.T) {
	var output bytes.Buffer
	log := slog.New(slogpretty.NewHandler(&output, nil)).With(
		slog.String("tag", "parent"),
		slog.Group("details", slog.Int("first", 1)),
	)
	log.Info("original message",
		slog.String("tag", "record"),
		slog.Group("details", slog.Int("second", 2)),
		slog.String("msg", "user message"),
		slog.String("level", "user level"),
		slog.String("time", "user time"),
		slog.Uint64("large", ^uint64(0)),
		slog.Group("nested", slog.Int("same", 1), slog.Int("same", 2)),
	)
	text := output.String()
	for _, want := range []string{
		"INFO: original message",
		`"tag": "parent"`, `"tag": "record"`,
		`"first": 1`, `"second": 2`,
		`"msg": "user message"`, `"level": "user level"`, `"time": "user time"`,
		`"large": 18446744073709551615`,
		`"same": 1`, `"same": 2`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("lost %q in output:\n%s", want, text)
		}
	}
	if strings.Count(text, `"details":`) != 2 {
		t.Fatalf("duplicate groups were collapsed:\n%s", text)
	}
}

func TestPrettyPreservesTransformedBuiltins(t *testing.T) {
	var output bytes.Buffer
	handler := slogpretty.NewHandler(&output, &slogpretty.HandlerOptions{SlogOpts: &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) != 0 {
				return attr
			}
			switch attr.Key {
			case slog.TimeKey:
				return slog.Int64(slog.TimeKey, 123456789)
			case slog.LevelKey:
				return slog.Int(slog.LevelKey, 42)
			case slog.MessageKey:
				return slog.Bool(slog.MessageKey, false)
			default:
				return attr
			}
		},
	}})
	slog.New(handler).Info("redacted message")
	text := output.String()
	start := strings.IndexByte(text, '{')
	if start < 0 {
		t.Fatalf("transformed attributes disappeared: %s", text)
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(text[start:]), &fields); err != nil {
		t.Fatal(err)
	}
	message, ok := fields[slog.MessageKey].(bool)
	if !ok || message || fields[slog.TimeKey] != float64(123456789) || fields[slog.LevelKey] != float64(42) {
		t.Fatalf("transformed attributes corrupted: %v", fields)
	}
	if strings.Contains(text, "redacted message") || strings.Contains(text, "[00:00:00.000]") {
		t.Fatalf("original or fabricated metadata rendered: %s", text)
	}
}

func TestPrettyOmitsMissingTime(t *testing.T) {
	for _, remove := range []bool{false, true} {
		name := "zero record time"
		var stamp time.Time
		var opts *slogpretty.HandlerOptions
		if remove {
			name = "removed by ReplaceAttr"
			stamp = time.Date(2026, 9, 22, 11, 23, 45, 0, time.UTC)
			opts = &slogpretty.HandlerOptions{SlogOpts: &slog.HandlerOptions{
				ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
					if attr.Key == slog.TimeKey {
						return slog.Attr{}
					}
					return attr
				},
			}}
		}
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			handler := slogpretty.NewHandler(&output, opts)
			record := slog.NewRecord(stamp, slog.LevelInfo, "event", 0)
			if err := handler.Handle(context.Background(), record); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(output.String(), "INFO: event") {
				t.Fatalf("missing time was fabricated: %s", output.String())
			}
		})
	}
}

func TestPrettyConcurrentDerivedHandlers(t *testing.T) {
	var output bytes.Buffer
	root := slog.New(slogpretty.NewHandler(&output, nil))
	var wg sync.WaitGroup
	for worker := range 20 {
		wg.Go(func() {
			child := root.With("worker", worker).WithGroup("request").With("active", true)
			for index := range 25 {
				child.Info("concurrent event", "index", index)
			}
		})
	}
	wg.Wait()
	text := output.String()
	if strings.Count(text, "INFO: concurrent event") != 500 || strings.Count(text, `"active": true`) != 500 {
		t.Fatalf("lost or interleaved concurrent records:\n%s", text)
	}
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	return len(data) / 2, nil
}

func TestPrettyPropagatesWriterErrors(t *testing.T) {
	failure := errors.New("destination failed")
	for _, test := range []struct {
		name   string
		writer io.Writer
		want   error
	}{
		{name: "write failure", writer: failingWriter{err: failure}, want: failure},
		{name: "short write", writer: failingWriter{}, want: io.ErrShortWrite},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := slogpretty.NewHandler(test.writer, nil)
			record := slog.NewRecord(time.Time{}, slog.LevelInfo, "event", 0)
			if err := handler.Handle(context.Background(), record); !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
}
