package slogpretty_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/assurrussa/gologger/handlers/slogcontext"
	"github.com/assurrussa/gologger/handlers/slogpretty"
)

type countedValue struct {
	calls *int
	value slog.Value
}

func (v countedValue) LogValue() slog.Value { *v.calls++; return v.value }

func TestPrettyBuiltinGroupsMatchJSON(t *testing.T) {
	for _, inline := range []bool{false, true} {
		for _, lazy := range []bool{false, true} {
			t.Run(fmt.Sprintf("inline=%t/lazy=%t", inline, lazy), func(t *testing.T) {
				expected, expectedTrace, expectedCount := encodeBuiltinGroups(t, false, inline, lazy)
				actual, actualTrace, actualCount := encodeBuiltinGroups(t, true, inline, lazy)
				if actual != expected {
					t.Fatalf("got %s\nwant %s", actual, expected)
				}
				if !reflect.DeepEqual(actualTrace, expectedTrace) {
					t.Fatalf("callback traces differ:\n%v\n%v", actualTrace, expectedTrace)
				}
				if actualCount != expectedCount {
					t.Fatalf("LogValuer count changed: %d != %d", actualCount, expectedCount)
				}
			})
		}
	}
}

func builtinGroup(name string, resolutions *int) slog.Attr {
	return slog.Group(name,
		slog.Group("removed", slog.String("drop", "secret")),
		slog.String("level", "INFO"), slog.String("msg", "event"),
		slog.Group("metadata", slog.Int("same", 1), slog.Int("same", 2)),
		slog.Group("", slog.Uint64("large", ^uint64(0))),
		slog.Any("lazy_nested", countedValue{resolutions, slog.GroupValue(slog.Int("n", 3))}),
		slog.Any("object", json.RawMessage(`{"1":{"msg":"raw"},"same":1,"same":2}`)),
	)
}

func encodeBuiltinGroups(t *testing.T, pretty, inline, lazy bool) (string, []string, int) {
	t.Helper()
	var output bytes.Buffer
	var trace []string
	resolutions := 0
	name := "replacement"
	if inline {
		name = ""
	}
	replacement := builtinGroup(name, &resolutions)
	opts := &slog.HandlerOptions{ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
		trace = append(trace, fmt.Sprintf("%v/%s/%s", groups, attr.Key, attr.Value.Kind()))
		switch {
		case attr.Key == slog.TimeKey && attr.Value.Kind() == slog.KindTime:
			if lazy {
				return slog.Any(name, countedValue{&resolutions, replacement.Value})
			}
			return replacement
		case attr.Key == slog.LevelKey && attr.Value.Kind() == slog.KindAny,
			attr.Key == slog.MessageKey && attr.Value.String() == "original", attr.Key == "drop":
			return slog.Attr{}
		default:
			return attr
		}
	}}
	var handler slog.Handler = slog.NewJSONHandler(&output, opts)
	if pretty {
		handler = slogpretty.NewHandler(&output, &slogpretty.HandlerOptions{SlogOpts: opts})
	}
	handler = handler.WithAttrs([]slog.Attr{slog.String("time", "user time"), slog.String("attrs", "user scope")})
	record := slog.NewRecord(time.Date(2026, 9, 22, 11, 23, 45, 0, time.UTC), slog.LevelInfo, "original", 0)
	record.AddAttrs(slog.String("0", "user frame"), slog.String("metadata", "user metadata"),
		slog.String("1:time", "user transport key"), slog.Group("2:level", slog.String("3:msg", "nested transport key")))
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, output.Bytes()); err != nil {
		t.Fatalf("%v: %s", err, output.Bytes())
	}
	return compact.String(), trace, resolutions
}

func TestPrettyMetadataCallbackParity(t *testing.T) {
	var traces [2][]string
	var resolutions [2]int
	var pcs [1]uintptr
	runtime.Callers(1, pcs[:])
	for index := range traces {
		opts := &slog.HandlerOptions{AddSource: true, ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			traces[index] = append(traces[index], fmt.Sprintf("%v/%s/%s", groups, attr.Key, attr.Value.Kind()))
			if attr.Key == "replacement" {
				return slog.Any(attr.Key, countedValue{&resolutions[index], slog.StringValue("resolved")})
			}
			return attr
		}}
		var output bytes.Buffer
		var handler slog.Handler = slog.NewJSONHandler(&output, opts)
		if index == 1 {
			handler = slogpretty.NewHandler(&output, &slogpretty.HandlerOptions{SlogOpts: opts})
		}
		handler = handler.WithAttrs([]slog.Attr{slog.Any("bound", countedValue{&resolutions[index], slog.StringValue("cached")})})
		if resolutions[index] != 1 {
			t.Fatal("WithAttrs was not eager")
		}
		handler = handler.WithGroup("request").WithAttrs([]slog.Attr{slog.Int("id", 1)})
		for _, pc := range []uintptr{0, pcs[0]} {
			record := slog.NewRecord(time.Date(2026, 9, 22, 11, 23, 45, 0, time.UTC), slog.LevelInfo, "event", pc)
			record.AddAttrs(slog.Any("inline", countedValue{&resolutions[index], slog.StringValue("inline")}), slog.Int("replacement", 1))
			if err := handler.Handle(context.Background(), record); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !reflect.DeepEqual(traces[0], traces[1]) {
		t.Fatalf("callback traces differ:\n%v\n%v", traces[0], traces[1])
	}
	if resolutions != [2]int{5, 5} {
		t.Fatalf("unexpected LogValuer invocations: %v", resolutions)
	}
}

func TestPrettyUserMetadataAcrossScopes(t *testing.T) {
	for _, group := range []string{"", "request"} {
		for _, remove := range []bool{false, true} {
			t.Run(fmt.Sprintf("group=%s/remove=%t", group, remove), func(t *testing.T) {
				var output bytes.Buffer
				opts := &slog.HandlerOptions{}
				var stamp time.Time
				if remove {
					stamp = time.Date(2026, 9, 22, 11, 23, 45, 0, time.UTC)
					opts.ReplaceAttr = func(_ []string, attr slog.Attr) slog.Attr {
						if attr.Key == slog.TimeKey && attr.Value.Kind() == slog.KindTime {
							return slog.Attr{}
						}
						return attr
					}
				}
				handler := slogcontext.NewHandler(slogpretty.NewHandler(&output, &slogpretty.HandlerOptions{SlogOpts: opts}))
				handler = handler.WithAttrs([]slog.Attr{slog.String("time", "bound")}).WithGroup(group)
				ctx := slogcontext.WithValue(context.Background(), slog.String("time", "context"))
				record := slog.NewRecord(stamp, slog.LevelInfo, "event", 0)
				record.AddAttrs(slog.Group("", slog.String("time", "inline")), slog.Group("nested", slog.String("time", "nested")))
				if err := handler.Handle(ctx, record); err != nil {
					t.Fatal(err)
				}
				requireUserTimes(t, output.String())
				if !strings.HasPrefix(output.String(), "INFO: event") {
					t.Fatalf("unexpected timestamp: %s", output.String())
				}
			})
		}
	}
}

func requireUserTimes(t *testing.T, output string) {
	t.Helper()
	for _, value := range []string{"bound", "context", "inline", "nested"} {
		if !strings.Contains(output, `"time": "`+value+`"`) {
			t.Fatalf("lost %s: %s", value, output)
		}
	}
}

func TestPrettyRenamedMetadataDoesNotBecomeAnotherHeader(t *testing.T) {
	var output bytes.Buffer
	opts := &slog.HandlerOptions{ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
		if attr.Key == slog.TimeKey && attr.Value.Kind() == slog.KindTime {
			attr.Key = slog.LevelKey
			return attr
		}
		if attr.Key == slog.LevelKey && attr.Value.Kind() == slog.KindAny {
			return slog.Attr{}
		}
		return attr
	}}
	handler := slogpretty.NewHandler(&output, &slogpretty.HandlerOptions{SlogOpts: opts})
	record := slog.NewRecord(time.Date(2026, 9, 22, 11, 23, 45, 0, time.UTC), slog.LevelInfo, "event", 0)
	record.AddAttrs(slog.String("level", "user level"))
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output.String(), "event {") || !strings.Contains(output.String(), `"level": "2026-09-22T11:23:45Z"`) {
		t.Fatalf("renamed metadata was promoted: %s", output.String())
	}
}

func TestPrettyReplacementCanLogReentrantly(t *testing.T) {
	var output bytes.Buffer
	var logger *slog.Logger
	nested := false
	opts := &slog.HandlerOptions{ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
		if !nested && attr.Key == slog.MessageKey {
			nested = true
			logger.Info("nested event")
		}
		return attr
	}}
	logger = slog.New(slogpretty.NewHandler(&output, &slogpretty.HandlerOptions{SlogOpts: opts}))
	logger.Info("outer event")
	if !strings.Contains(output.String(), "nested event") || !strings.Contains(output.String(), "outer event") {
		t.Fatalf("lost reentrant log: %s", output.String())
	}
}

func TestPrettyPreservesLazilyEmptyBuiltinGroups(t *testing.T) {
	for _, inline := range []bool{false, true} {
		t.Run(fmt.Sprintf("inline=%t", inline), func(t *testing.T) {
			var output bytes.Buffer
			calls := 0
			name := "child"
			if inline {
				name = ""
			}
			opts := &slog.HandlerOptions{ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
				switch attr.Key {
				case slog.TimeKey:
					return slog.Group("parent", slog.Any(name, countedValue{&calls, slog.GroupValue()}))
				case slog.LevelKey, slog.MessageKey:
					return slog.Attr{}
				default:
					return attr
				}
			}}
			handler := slogpretty.NewHandler(&output, &slogpretty.HandlerOptions{SlogOpts: opts})
			record := slog.NewRecord(time.Date(2026, 9, 22, 11, 23, 45, 0, time.UTC), slog.LevelInfo, "event", 0)
			if err := handler.Handle(context.Background(), record); err != nil {
				t.Fatal(err)
			}
			var compact bytes.Buffer
			if err := json.Compact(&compact, output.Bytes()); err != nil {
				t.Fatal(err)
			}
			if compact.String() != `{"parent":{}}` || calls != 1 {
				t.Fatalf("lost empty group: %s; calls=%d", output.String(), calls)
			}
		})
	}
}

func TestPrettyRenamedSourceStaysAnAttribute(t *testing.T) {
	var output bytes.Buffer
	opts := &slog.HandlerOptions{AddSource: true, ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
		switch {
		case attr.Key == slog.SourceKey:
			return slog.String(slog.MessageKey, "caller")
		case attr.Key == slog.LevelKey, attr.Key == slog.MessageKey && attr.Value.String() == "redacted":
			return slog.Attr{}
		default:
			return attr
		}
	}}
	handler := slogpretty.NewHandler(&output, &slogpretty.HandlerOptions{SlogOpts: opts})
	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "redacted", 0)
	record.AddAttrs(slog.String(slog.MessageKey, "user message"))
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, output.Bytes()); err != nil {
		t.Fatal(err)
	}
	if compact.String() != `{"msg":"caller","msg":"user message"}` {
		t.Fatal(output.String())
	}
}
