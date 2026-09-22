package gologger

import (
	"bufio"
	"errors"
	"os"
	"sync"
)

// fileSink serializes writes and shutdown so a closed sink cannot buffer data.
type fileSink struct {
	file   *os.File
	buffer *bufio.Writer
	mu     sync.Mutex
	closed bool
	err    error
}

func newFileSink(file *os.File, size int) *fileSink {
	return &fileSink{file: file, buffer: bufio.NewWriterSize(file, size)}
}

func (s *fileSink) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, os.ErrClosed
	}
	return s.buffer.Write(data)
}

func (s *fileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		s.err = errors.Join(s.buffer.Flush(), s.file.Sync(), s.file.Close())
	}
	return s.err
}
