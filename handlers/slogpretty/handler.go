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
	var options slog.HandlerOptions
	if opts != nil && opts.SlogOpts != nil {
		options = *opts.SlogOpts
	}
	replace := options.ReplaceAttr
	if replace == nil {
		encoder := slog.NewJSONHandler(&prettyWriter{writer: writer}, &options).WithGroup("attrs")
		return &Handler{Handler: encoder}
	}
	options.ReplaceAttr = func(groups []string, attr slog.Attr) slog.Attr {
		if len(groups) != 0 {
			return replace(groups[1:], attr)
		}
		// Internal carrier callbacks must not repeat the user's replacement.
		switch value := attr.Value.Any().(type) {
		case replacedAttr:
			return value.attr
		case frameEnd:
			return attr
		case groupAttr:
			attr = value.attr
			attr.Value = attr.Value.Resolve()
			if attr.Value.Kind() == slog.KindGroup {
				return frameMetadata("", attr)
			}
		}
		originalKey := attr.Key
		attr = replace(nil, attr)
		attr.Value = attr.Value.Resolve()
		return frameMetadata(originalKey, attr)
	}
	// The standard handler keeps every built-in subtree outside this scope.
	// User keys can therefore never be mistaken for framed metadata.
	encoder := slog.NewJSONHandler(&prettyWriter{writer: writer, framed: true}, &options).WithGroup("attrs")
	return &Handler{Handler: encoder}
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{Handler: h.Handler.WithGroup(name)}
}

type prettyWriter struct {
	writer io.Writer
	framed bool
}

func (w *prettyWriter) Write(data []byte) (int, error) {
	fields, headers, err := decodeRecord(data, w.framed)
	if err != nil {
		return 0, fmt.Errorf("decode log record: %w", err)
	}
	stamp := headers[0]
	if parsed, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
		stamp = parsed.Format("15:04:05.000")
	}
	level := headers[1]
	message := headers[2]
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
