package jsonl

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"unicode/utf8"
)

var ErrNotJSON = fmt.Errorf("argument to Write() was not valid JSON")

// Open a file as jsonl. The returned jsonl struct implements
// io.ReadWriteCloser, thus Close() should be called when the
// data store is no longer needed.
//
// Concurrent Read()s and Write()s are not supported as to
// prevent data access race conditions.
func Open(f *os.File) (*Jsonl, error) {
	if f == nil {
		return nil, os.ErrNotExist
	}
	return &Jsonl{
		f:  f,
		mu: &sync.Mutex{},
	}, nil
}

// OpenFile is a convenience method for opening a jsonl file
func OpenFile(filename string) (*Jsonl, error) {
	// Append fsync'd writes
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	j, err := Open(f)
	if err != nil {
		return nil, err
	}
	j.f = f
	return j, nil
}

var _ io.ReadWriteCloser = &Jsonl{}

// Jsonl is a mutex-protect jsonl file which implements io.ReadWriteCloser.
type Jsonl struct {
	f  *os.File
	mu *sync.Mutex
}

// Close the jsonl file.
func (j *Jsonl) Close() error {
	return j.f.Close()
}

// Read the latest non-corrupt jsonl entry into p.
func (j *Jsonl) Read(p []byte) (int, error) {
	if j.f == nil {
		return 0, os.ErrNotExist
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	stat, err := j.f.Stat()
	if err != nil {
		return 0, err
	}
	buf := make([]byte, stat.Size())
	n, err := j.f.ReadAt(buf, 0)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			return 0, err
		}
	}
	if n == 0 {
		return 0, nil
	}
	buf = buf[:n-1]
	start := -1
	end := -1
	for i := len(buf) - 1; i >= 0; i-- {
		if buf[i] == '\n' {
			end = start
			start = i
		}
	}
	if end < 0 || start < 0 {
		return 0, nil
	}
	return copy(p, buf[start:end]), nil
}

// Write the JSON byte slice p to the jsonl file.
func (j *Jsonl) Write(p []byte) (n int, err error) {
	if !utf8.Valid(p) {
		return 0, ErrNotJSON
	}
	if !json.Valid(p) {
		return 0, ErrNotJSON
	}
	p = bytes.TrimSpace(p)
	if p[len(p)-1] != '\n' {
		p = append(p, '\n')
	}
	// Prior to performing a write, we must check that the last
	// write completed successfully. If the last character in the
	// file is not a newline, we must inject one on the next write
	// to make a valid entry.
	j.mu.Lock()
	defer j.mu.Unlock()
	stat, err := j.f.Stat()
	if err != nil {
		return 0, err
	}
	buf := make([]byte, 1)
	n, err = j.f.ReadAt(buf, stat.Size()-1)
	if n > 0 {
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return 0, err
			}
		}
		if buf[0] != '\n' {
			p = append([]byte("\n"), p...)
		}
	}
	return j.f.Write(p)
}
