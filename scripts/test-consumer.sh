#!/bin/sh
set -eu

version=${1:?usage: test-consumer.sh published-version-or-commit}
consumer_dir=$(mktemp -d "${TMPDIR:-/tmp}/gologger-consumer.XXXXXX")
trap 'rm -rf "$consumer_dir"' EXIT HUP INT TERM
export GOWORK=off
cd "$consumer_dir"
go mod init example.com/gologger-consumer
go get "github.com/assurrussa/gologger@$version"
cat > consumer_test.go <<'EOF'
package consumer_test

import (
    "bytes"
    "context"
    "encoding/json"
    "log/slog"
    "testing"

    "github.com/assurrussa/gologger"
    "github.com/assurrussa/gologger/handlers/slogcontext"
    "github.com/assurrussa/gologger/handlers/slogdiscard"
    "github.com/assurrussa/gologger/handlers/slogpretty"
)

func TestPublicAPI(t *testing.T) {
    previous := slog.Default()
    var output bytes.Buffer
    log, err := gologger.New(gologger.Config{Level: "debug"}, gologger.WithWriter(&output))
    if err != nil { t.Fatal(err) }
    var contract gologger.Logger = log
    ctx := slogcontext.WithValue(context.Background(), slog.String("request_id", "external"))
    contract.WithNamed("consumer").InfoContext(ctx, "ready")
    if err := log.Close(); err != nil { t.Fatal(err) }
    var record map[string]any
    if err := json.Unmarshal(output.Bytes(), &record); err != nil { t.Fatal(err) }
    if record["request_id"] != "external" || record["name"] != "consumer" { t.Fatalf("lost attributes: %v", record) }
    if slog.Default() != previous { t.Fatal("constructor changed process default") }
    var _ slog.Handler = slogpretty.NewHandler(&output, nil)
    var _ slog.Handler = slogdiscard.NewHandler()
}
EOF
go mod tidy
go test -race ./...
if go list -m all | grep -q 'github.com/assurrussa/goshared'; then
    echo 'unexpected goshared dependency' >&2
    exit 1
fi
go list -m github.com/assurrussa/gologger
