// Package slogpretty formats slog's JSON output for local interactive use.
package slogpretty

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
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
	fields, err := decodeFields(data)
	if err != nil {
		return 0, fmt.Errorf("decode log record: %w", err)
	}
	stamp := fields.takeString(slog.TimeKey)
	if parsed, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
		stamp = parsed.Format("15:04:05.000")
	}
	level := fields.takeString(slog.LevelKey)
	message := fields.takeString(slog.MessageKey)
	attrs, err := fields.indented()
	if err != nil {
		return 0, fmt.Errorf("encode log fields: %w", err)
	}
	parts := make([]string, 0, 4)
	if stamp != "" {
		parts = append(parts, "["+stamp+"]")
	}
	if level != "" {
		parts = append(parts, colorLevel(level))
	}
	if message != "" {
		parts = append(parts, color.CyanString(message))
	}
	if len(attrs) > 0 {
		parts = append(parts, color.WhiteString(string(attrs)))
	}
	line := strings.Join(parts, " ") + "\n"
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
