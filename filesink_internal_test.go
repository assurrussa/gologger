package gologger

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestFileSinkConcurrentCloseAndWrite(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "log"))
	if err != nil {
		t.Fatal(err)
	}
	sink := newFileSink(file, 1024)
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			_, err := sink.Write([]byte("record\n"))
			if err != nil && !errors.Is(err, os.ErrClosed) {
				t.Error(err)
			}
		})
		wg.Go(func() {
			if err := sink.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if _, err := sink.Write([]byte("lost")); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("write after close was buffered: %v", err)
	}
}
