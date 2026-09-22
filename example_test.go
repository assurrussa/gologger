package gologger_test

import (
	"context"
	"log/slog"
	"os"

	"github.com/assurrussa/gologger"
)

func ExampleNew() {
	log, err := gologger.New(gologger.Config{Level: "info"},
		gologger.WithWriter(os.Stdout),
		gologger.WithReplaceAttr(func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return attr
		}))
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			panic(err)
		}
	}()
	ctx := gologger.WithValue(context.Background(), slog.String("request_id", "r-1"))
	log.WithNamed("worker").InfoContext(ctx, "ready")
	// Output: {"level":"INFO","msg":"ready","name":"worker","request_id":"r-1"}
}
