package slogcontext_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"testing/slogtest"

	"github.com/assurrussa/gologger/handlers/slogcontext"
)

func TestSiblingContextsAreIndependent(t *testing.T) {
	parent := context.Background()
	for i := range 3 {
		parent = slogcontext.WithValue(parent, slog.Int(fmt.Sprintf("parent%d", i), i))
	}
	var wg sync.WaitGroup
	for i := range 40 {
		wg.Go(func() {
			ctx := slogcontext.WithValue(parent, slog.Int("child", i))
			var output bytes.Buffer
			log := slog.New(slogcontext.NewHandler(slog.NewJSONHandler(&output, nil))).WithGroup("request")
			log.InfoContext(ctx, "event")
			var record struct {
				Request map[string]int `json:"request"`
			}
			if err := json.Unmarshal(output.Bytes(), &record); err != nil {
				t.Error(err)
				return
			}
			if record.Request["child"] != i || len(record.Request) != 4 {
				t.Errorf("sibling context modified: %+v", record)
			}
		})
	}
	wg.Wait()
}

func TestHandlerConformance(t *testing.T) {
	var output bytes.Buffer
	handler := slogcontext.NewHandler(slog.NewJSONHandler(&output, nil))
	if err := slogtest.TestHandler(handler, func() []map[string]any {
		decoder := json.NewDecoder(&output)
		var records []map[string]any
		for decoder.More() {
			var record map[string]any
			if err := decoder.Decode(&record); err != nil {
				t.Fatal(err)
			}
			records = append(records, record)
		}
		return records
	}); err != nil {
		t.Fatal(err)
	}
}
