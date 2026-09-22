# gologger

[Русский](README.md) | **English**

A configurable logger built on the standard `log/slog` package: JSON and readable
text output, context attributes, multiple destinations, and extensions through
custom handlers and middleware.

Requires **Go 1.27+**. Recommended toolchain: **Go 1.27.1**.
Uses the standard
[`slog.NewMultiHandler`](https://pkg.go.dev/log/slog@go1.27.0#NewMultiHandler)
and `slog.DiscardHandler`.

## Installation

```sh
go get github.com/assurrussa/gologger@latest
```

Import path: `github.com/assurrussa/gologger`; package name: `gologger`.

## Quick start

The following is the body of a function returning `error`. Required imports:
`context`, `log/slog`, `os`, and `github.com/assurrussa/gologger`.

```go
log, err := gologger.New(gologger.Config{Level: "info"},
    gologger.WithWriter(os.Stdout),
)
if err != nil {
    return err
}

ctx := gologger.WithValue(context.Background(), slog.String("request_id", "req-1"))
log.WithNamed("worker").InfoContext(ctx, "started")
return log.Close()

```

In an application, call `Close` after all users of the logger have finished.
The [executable example](example_test.go) is also checked by `go test ./...`.

`New` creates an independent instance: importing the package and constructing a
logger do not change `LogLevel` or `slog.Default()`. To configure the application
default, explicitly call `gologger.SetDefault(log)`.

## Configuration

Behavior of `gologger.New` without additional options:

| `Config` field | Default | Behavior |
| --- | --- | --- |
| `Level` | WARN for an empty string | Minimum level. An invalid value returns an error. |
| `Env` | Empty string | `local` and `development` enable readable pretty output; other values select JSON. |
| `JSON` | `false` | `true` forces JSON even in a local environment. |
| `AddSource` | `false` | Includes the caller's file and line. |
| `AddVerbose` | `false` | Adds `program_info`: PID, application version, and Go version. |
| `Rate` | `0` | Probability of keeping a record when `0 < Rate < 1`; `0` and `1` keep all records. |
| `Output` | Empty string | Path to an additional buffered JSON file. |

The primary output goes to stdout. `Config.Output` adds a file and keeps the
primary output, including when a custom handler is supplied.

`New` does not read environment variables or apply the `value-default` and
`validate` tags itself. The tags in `Config` are intended for the application's
configuration layer. Options override configuration values.

## Customization and extensions

- `WithWriter(io.Writer)` replaces stdout for the built-in handler.
- `WithHandler(slog.Handler)` replaces the built-in handler. The supplied
  handler controls its own level, `AddSource`, and `ReplaceAttr` settings.
- `WithAdditionalHandlers(...OptionHandler)` adds destinations. Each factory
  receives its own copy of `slog.HandlerOptions`; a `nil` result is skipped.
- `WithMiddleware(...Middleware)` wraps the combined handler. Middleware runs
  in the supplied order: the first wrapper is the outermost.
- `WithLevel(slog.Leveler)` overrides `Config.Level` without mutating the supplied
  value. A `*slog.LevelVar` supports runtime level changes.
- `WithReplaceAttr` configures attribute filtering, renaming, or redaction for
  built-in handlers and additional handler factories.
- `WithClosers(...io.Closer)` transfers responsibility for closing additional
  resources after successful construction.

This example combines a dynamic level, an additional stderr destination,
attribute redaction, and middleware. Required imports: `log/slog`, `os`,
and `github.com/assurrussa/gologger`.

```go
level := new(slog.LevelVar)
level.Set(slog.LevelDebug)

log, err := gologger.New(gologger.Config{},
    gologger.WithLevel(level),
    gologger.WithAdditionalHandlers(func(opts *slog.HandlerOptions) slog.Handler {
        return slog.NewTextHandler(os.Stderr, opts)
    }),
    gologger.WithReplaceAttr(func(_ []string, attr slog.Attr) slog.Attr {
        if attr.Key == "password" {
            return slog.String(attr.Key, "[redacted]")
        }
        return attr
    }),
    gologger.WithMiddleware(func(next slog.Handler) slog.Handler {
        return next.WithAttrs([]slog.Attr{slog.String("service", "api")})
    }),
)
if err != nil {
    return err
}
log.Info("ready", "password", "example")
level.Set(slog.LevelWarn)
return log.Close()

```

`WithLevel` affects built-in handlers and factories that use the supplied options.
A handler passed through `WithHandler` controls its own threshold. Changing a
`LevelVar` affects only loggers explicitly configured with that pointer.

Processing order: **context attributes → middleware → sampling → destinations**.
Handlers and middleware must preserve the `slog.Handler` contract, including
`Enabled`, `WithAttrs`, `WithGroup`, and concurrent access safety. If several
independent handlers share a writer, the caller must synchronize that writer.

## Context and derived loggers

`WithValue(ctx, slog.Attr)` creates a context containing an additional attribute.
Pass it to `InfoContext`, `DebugContext`, `WarnContext`, `ErrorContext`, or
`LogAttrs` to include the attribute in a record. Sibling contexts do not mutate
each other's attributes.

`WithNamed` adds the `name` field; `WithAttrs` adds persistent attributes. Both
return the `Logger` interface and share resources with the parent. The embedded
`*slog.Logger` is available through the `Logger` field; inherited `With` and
`WithGroup` methods return a regular `*slog.Logger`. The original `*gologger.Log`
remains responsible for closing resources.

`Error(err)` creates a string attribute named `error`; pass a non-nil error.
`Discard` creates a logger with no output; `DiscardJSONWithWriter` and
`DiscardTextWithWriter` create loggers for the supplied writer. These functions
use the shared `LogLevel`.

## Sampling

For `0 < Rate < 1`, the library makes one random decision per record, so all
destinations receive the same selected records. Sampling applies to every level,
including ERROR. Values `0` and `1` disable sampling.

Negative values, values greater than `1`, and NaN cause `New` to return an error.

## Public handlers

All paths below are prefixed with `github.com/assurrussa/gologger/`:

- `handlers/slogcontext` adds attributes from the context.
- `handlers/slogpretty` provides readable local output with groups, `LogValuer`,
  `ReplaceAttr`, and caller source information.
- `handlers/slogdiscard` discards records.

Handlers work with a regular `slog.New` without a `gologger.Log` wrapper.

## Resources and shutdown

`Config.Output` creates new files with mode `0600`; permissions of existing files
are unchanged. Records are buffered and persisted when the logger is closed.

`Close()` flushes and closes owned resources once. `Flush()` is a compatibility
name for the same terminal operation, not a periodic buffer flush. All calls,
including concurrent calls through derived loggers, return the stored close
error. Stop the logger's users before closing it; logging after close is unsupported.

Supplied writers and handlers remain owned by the caller unless ownership is
explicitly transferred, and are not closed by the library. `WithClosers` transfers
responsibility only after construction succeeds. Resources close in reverse
order; each `io.Closer` must flush its own buffers. The library starts no goroutines
or background timers.

## Global logger

`SetDefault(log)` updates both the package default and `slog.Default()`. The
previous instance remains owned by the application and is not closed automatically.
`Default()` returns the current package logger.

`NewLogger` creates and installs a global logger, updates the shared `LogLevel`,
falls back to WARN for an invalid level, ignores `JSON` in `local`/`development`,
and treats rates outside `(0, 1)` as disabled sampling. Use `New` for independent
instances. Change the shared `LogLevel` through `.Set`; do not replace its pointer.

## Validation

Requires Go 1.27+ and `golangci-lint`:

```sh
make check
./scripts/test-consumer.sh <published-version-or-commit>
```

`make check` runs vet, lint, and race tests five times. The consumer probe creates
a separate Go module with `GOWORK=off`, downloads the specified published version
without `replace` directives, and checks the public imports with the race detector.

## License

[MIT](LICENSE).
