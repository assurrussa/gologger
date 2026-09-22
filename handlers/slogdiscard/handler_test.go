package slogdiscard_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/assurrussa/gologger/handlers/slogdiscard"
)

func TestNewDiscardHandler(t *testing.T) {
	ctx := context.Background()
	logHandler := slogdiscard.NewHandler()

	require.NoError(t, logHandler.Handle(ctx, slog.Record{}))
	assert.False(t, logHandler.Enabled(ctx, slog.LevelWarn))

	newLogHandler := logHandler.WithAttrs([]slog.Attr{}).WithGroup("test")
	assert.NotNil(t, newLogHandler)
	assert.Equal(t, newLogHandler, logHandler)
}
