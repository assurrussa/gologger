package slogcontext_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/assurrussa/gologger/handlers/slogcontext"
)

func TestNewDiscardHandler(t *testing.T) {
	bf := bytes.NewBuffer(nil)
	logHandler := slogcontext.NewHandler(slog.NewTextHandler(bf, nil))

	//nolint:staticcheck // test
	ctx := slogcontext.WithValue(nil, slog.String("test", "val1"))
	ctx = slogcontext.WithValue(ctx, slog.String("test2", "val2"))

	require.NoError(t, logHandler.Handle(ctx, slog.Record{Message: "test message"}))
	assert.True(t, logHandler.Enabled(ctx, slog.LevelWarn))

	newLogHandler := logHandler.WithAttrs([]slog.Attr{}).WithGroup("test")
	assert.NotNil(t, newLogHandler)
	assert.Equal(t, `level=INFO msg="test message" test=val1 test2=val2
`, bf.String())
}
