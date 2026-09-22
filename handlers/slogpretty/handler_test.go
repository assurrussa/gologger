package slogpretty_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/assurrussa/gologger/handlers/slogpretty"
)

type secret string

func (secret) LogValue() slog.Value { return slog.StringValue("[hidden]") }

func TestPrettyPreservesSlogSemantics(t *testing.T) {
	var output bytes.Buffer
	handler := slogpretty.NewHandler(&output, &slogpretty.HandlerOptions{SlogOpts: &slog.HandlerOptions{
		AddSource: true,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == "token" {
				return slog.String("token", "[redacted]")
			}
			return attr
		},
	}})
	log := slog.New(handler).With("root", true).WithGroup("request").With("count", 42)
	log.Info(
		"event",
		"credential", secret("plaintext"),
		"token", "token-value",
		"nested", slog.GroupValue(slog.Bool("valid", true)),
	)
	text := output.String()
	if strings.Contains(text, "plaintext") || strings.Contains(text, "token-value") {
		t.Fatalf("unresolved secret: %s", text)
	}
	start := strings.IndexByte(text, '{')
	if start < 0 {
		t.Fatalf("no fields: %s", text)
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(text[start:]), &fields); err != nil {
		t.Fatal(err)
	}
	request, ok := fields["request"].(map[string]any)
	root, rootOK := fields["root"].(bool)
	if !ok || !rootOK || !root ||
		request["count"] != float64(42) ||
		request["credential"] != "[hidden]" ||
		request["token"] != "[redacted]" {
		t.Fatalf("group or value corrupted: %v", fields)
	}
	if fields["source"] == nil {
		t.Fatal("source not rendered")
	}
}

func TestPrettyNilOptionsAndTime(t *testing.T) {
	var output bytes.Buffer
	handler := slogpretty.NewHandler(&output, nil)
	record := slog.NewRecord(time.Date(2026, 9, 22, 11, 23, 45, 0, time.UTC), slog.LevelInfo, "event", 0)
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output.String(), "[11:23:45.000] INFO: event") {
		t.Fatalf("incorrect timestamp: %s", output.String())
	}
}
