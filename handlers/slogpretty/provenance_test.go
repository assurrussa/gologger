package slogpretty_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/assurrussa/gologger/handlers/slogpretty"
)

func TestPrettyMetadataDoesNotConsumeUserFields(t *testing.T) {
	stamp := time.Date(2026, 9, 22, 11, 23, 45, 0, time.UTC)
	for _, key := range []string{slog.TimeKey, slog.LevelKey, slog.MessageKey} {
		for _, change := range []string{"remove", "rename", "number"} {
			t.Run(key+"/"+change, func(t *testing.T) {
				var output bytes.Buffer
				replace := func(_ []string, attr slog.Attr) slog.Attr {
					if attr.Key != key || attr.Value.Kind() == slog.KindString && attr.Value.String() != "event" {
						return attr
					}
					switch change {
					case "remove":
						return slog.Attr{}
					case "rename":
						attr.Key = "renamed_" + key
					case "number":
						attr.Value = slog.IntValue(42)
					}
					return attr
				}
				handler := slogpretty.NewHandler(&output, &slogpretty.HandlerOptions{SlogOpts: &slog.HandlerOptions{ReplaceAttr: replace}})
				record := slog.NewRecord(stamp, slog.LevelInfo, "event", 0)
				record.AddAttrs(slog.String(key, "2026-09-22T11:23:45+05:00"))
				if err := handler.Handle(context.Background(), record); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(output.String(), `"`+key+`": "2026-09-22T11:23:45+05:00"`) {
					t.Fatalf("user field promoted or lost:\n%s", output.String())
				}
			})
		}
	}
	t.Run("zero time", func(t *testing.T) {
		var output bytes.Buffer
		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "event", 0)
		record.AddAttrs(slog.String(slog.TimeKey, "2026-09-22T11:23:45+05:00"))
		if err := slogpretty.NewHandler(&output, nil).Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(output.String(), "INFO: event {") ||
			!strings.Contains(output.String(), `"time": "2026-09-22T11:23:45+05:00"`) {
			t.Fatalf("user time promoted or lost:\n%s", output.String())
		}
	})
}

func BenchmarkPrettyProvenance(b *testing.B) {
	for _, replace := range []bool{false, true} {
		name := "plain"
		opts := &slogpretty.HandlerOptions{SlogOpts: &slog.HandlerOptions{}}
		if replace {
			name = "replace"
			opts.SlogOpts.ReplaceAttr = func(_ []string, attr slog.Attr) slog.Attr { return attr }
		}
		b.Run(name, func(b *testing.B) {
			handler := slogpretty.NewHandler(io.Discard, opts).WithAttrs([]slog.Attr{slog.String("component", "worker")})
			record := slog.NewRecord(time.Date(2026, 9, 22, 11, 23, 45, 0, time.UTC), slog.LevelInfo, "event", 0)
			record.AddAttrs(slog.String("time", "user time"), slog.Int("count", 42))
			b.ReportAllocs()
			for b.Loop() {
				if err := handler.Handle(context.Background(), record); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
