package gologger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestSamplingPreservedAcrossDerivationAndDestinations(t *testing.T) {
	var first, second bytes.Buffer
	draw := 0.1
	h := &samplingHandler{
		Handler: slog.NewMultiHandler(slog.NewJSONHandler(&first, nil), slog.NewJSONHandler(&second, nil)),
		rate:    0.5, random: func() float64 { return draw },
	}
	log := slog.New(h).With("root", "value").WithGroup("request").With("id", "req")
	log.InfoContext(context.Background(), "keep")
	draw = 0.5
	log.InfoContext(context.Background(), "drop")
	if first.String() != second.String() ||
		!strings.Contains(first.String(), `"request":{"id":"req"}`) ||
		!strings.Contains(first.String(), "keep") ||
		strings.Contains(first.String(), "drop") {
		t.Fatalf("sampling or attributes lost: %s / %s", first.String(), second.String())
	}
}
