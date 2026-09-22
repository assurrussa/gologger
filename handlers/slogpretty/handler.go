// Package slogpretty formats slog's JSON output for local interactive use.
package slogpretty

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/fatih/color"
)

// HandlerOptions configures the standard slog encoding used before rendering.
type HandlerOptions struct {
	SlogOpts *slog.HandlerOptions
}

// Handler delegates attribute resolution, grouping, source and ReplaceAttr to
// slog.JSONHandler. Its shared lock also serializes writes from derived handlers.
type Handler struct {
	slog.Handler
}

func NewHandler(writer io.Writer, opts *HandlerOptions) *Handler {
	var slogOpts *slog.HandlerOptions
	if opts != nil {
		slogOpts = opts.SlogOpts
	}
	return &Handler{Handler: slog.NewJSONHandler(&prettyWriter{writer: writer}, slogOpts)}
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{Handler: h.Handler.WithGroup(name)}
}

type prettyWriter struct {
	writer io.Writer
}

func (w *prettyWriter) Write(data []byte) (int, error) {
	fields := make(map[string]any)
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&fields); err != nil {
		return 0, fmt.Errorf("decode log record: %w", err)
	}
	stamp, _ := fields[slog.TimeKey].(string)
	if stamp == "" {
		stamp = "00:00:00.000"
	}
	if parsed, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
		stamp = parsed.Format("15:04:05.000")
	}
	level, _ := fields[slog.LevelKey].(string)
	message, _ := fields[slog.MessageKey].(string)
	delete(fields, slog.TimeKey)
	delete(fields, slog.LevelKey)
	delete(fields, slog.MessageKey)
	var attrs []byte
	if len(fields) > 0 {
		var err error
		attrs, err = json.MarshalIndent(fields, "", "  ")
		if err != nil {
			return 0, fmt.Errorf("encode log fields: %w", err)
		}
	}
	line := fmt.Sprintf(
		"[%s] %s %s %s\n",
		stamp,
		colorLevel(level),
		color.CyanString(message),
		color.WhiteString(string(attrs)),
	)
	written, err := io.WriteString(w.writer, line)
	if err != nil {
		return 0, err
	}
	if written != len(line) {
		return 0, io.ErrShortWrite
	}
	return len(data), nil
}

func colorLevel(level string) string {
	switch level {
	case slog.LevelDebug.String():
		return color.MagentaString(level + ":")
	case slog.LevelInfo.String():
		return color.BlueString(level + ":")
	case slog.LevelWarn.String():
		return color.YellowString(level + ":")
	case slog.LevelError.String():
		return color.RedString(level + ":")
	default:
		return level + ":"
	}
}
