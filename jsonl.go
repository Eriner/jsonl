/*
Package jsonl reads and writes to .jsonl files.

Each *Jsonl{} returned by Open() or OpenFile() is a handle to
a single file and you must call Close() to release.

The Read() method returns the last non-corrupt JSON entry.
Thus different types should be written to their own *Jsonl{}.

*Jsonl{} is safe for concurrent access.

*/
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

const entrySizeCap int64 = 1024 * 1024 * 16 // 16M
// ErrNotJSON is returned if the argument passed to Write() was
// not valid JSON.
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

func (j *Jsonl) Decode(v interface{}) error {
	if j.f == nil {
		return os.ErrNotExist
	}
	stat, err := j.f.Stat()
	if err != nil {
		return err
	}
	if stat.Size() == 0 {
		// Empty file, nothing to decode.
		return nil
	}
	dec := json.NewDecoder(j)
	return dec.Decode(v)
}

func (j *Jsonl) Encode(v interface{}) error {
	if j.f == nil {
		return os.ErrNotExist
	}
	enc := json.NewEncoder(j)
	return enc.Encode(v)
}

// Read the latest non-corrupt jsonl entry into p.
func (j *Jsonl) Read(p []byte) (int, error) {
	const chunkSize int64 = 4096 // 4K
	if j.f == nil {
		return 0, os.ErrNotExist
	}
	stat, err := j.f.Stat()
	if err != nil {
		return 0, err
	}
	buf := make([]byte, chunkSize, entrySizeCap)
	// than this, you should probably be using a database.
	start := -1
	end := -1
	for off := stat.Size() / chunkSize; off >= 0; off = off - chunkSize {
		n, err := j.f.ReadAt(buf, off*chunkSize)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return 0, fmt.Errorf("jsonl failed reading the underlying file: %w", err)
			}
		}
		if n == 0 {
			return 0, nil
		}
		// Regardless of what chunk we're processing, we read backwards over the
		// whole buf. While this is somewhat inefficient, in-memory manipulations
		// are fast.
		for i := len(buf) - 1; i >= 0; i-- {
			if buf[i] == '\n' {
				end = start
				start = i
			}
			if end > 0 {
				break
			}
		}
		if end > 0 {
			break
		}
		buf = buf[:n] // will over-allocate in some circumstances, but that's fine.
	}
	// Handle the first entry not having a newline
	if end < 0 && start > -1 {
		if stat.Size()/chunkSize == 0 {
			end = start
			start = 0
		} else {
			return 0, fmt.Errorf("jsonl: entry exceeded 16M size limit")
		}
	}
	if end < 0 || start < 0 {
		return 0, io.EOF
	}
	return copy(p, buf[start:end]), nil
}

// Write the JSON byte slice p to the jsonl file.
func (j *Jsonl) Write(p []byte) (n int, err error) {
	if j.f == nil {
		return 0, os.ErrNotExist
	}
	if int64(len(p)) > entrySizeCap {
		return 0, fmt.Errorf("jsonl: data passed to write exceeds the 16M entry size limit")
	}
	// TODO: This function is messy and makes a lot of unnecessary allocations.
	// My use-cases aren't performance intensive, so this is fine. Ideally I
	// would write benchmarks and optimize.
	if !utf8.Valid(p) {
		return 0, ErrNotJSON
	}
	if !json.Valid(p) {
		return 0, ErrNotJSON
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, bytes.TrimSpace(p)); err != nil {
		return 0, ErrNotJSON
	}
	p = buf.Bytes()
	// Append single newline at the end of the buf
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
	lr := make([]byte, 1)
	n, err = j.f.ReadAt(lr, stat.Size()-1)
	if n > 0 {
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return 0, fmt.Errorf("jsonl failed to read the last byte of file before Write(): %w", err)
			}
		}
		if lr[0] != '\n' {
			p = append([]byte("\n"), p...)
		}
	}
	n, err = j.f.Write(p)
	if err != nil {
		return n, err
	}
	return n, j.f.Sync()
}
