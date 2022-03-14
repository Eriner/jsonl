// Package jsonl implements JSON Lines (.jsonl) in Go.
//
// "The JSON Lines format has three requirements"
// "1. UTF-8 Encoding"
// "2. Each line is a valid JSON value"
// "3. Line separator is '\n'"
//
// Ref: https://jsonlines.org/
//
package jsonl

// TODO:
// * configureable scanner bufsize, don't truncate. Add tests for length.
// * add opts: WithGzip, WithMaxEntryLen, WithFsync, etc.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"unicode/utf8"
)

var (
	ErrNotUTF8       = fmt.Errorf("not utf8")
	ErrNotJSON       = fmt.Errorf("not valid JSON")
	ErrEntryNotFound = fmt.Errorf("jsonl entry not found")
)

// File returns a file-backed jsonl store. Files are opened
// in append-only mode. If the file does not exist, it will
// be created.
//
// If you need to ensure writes persist in the event of a
// power-failure (think embedded devices), use WithFsync.
// To use Gzip, use WithGzip.
//
// Opening a file reads the entire file to count the number of
// entries.
//
// TODO: keep a "appended" and "on-disk" count and merge,
// lazy-reading to init the latter.
func File(filename string) (*jsonl, error) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o600) // append writes
	if err != nil {
		return nil, err
	}
	j, err := Open(f)
	if err != nil {
		return nil, err
	}
	j.name = filename
	j.file = f
	return j, nil
}

// Open reads and writes jsonl
func Open(r io.ReadWriteCloser) (*jsonl, error) {
	scanner := bufio.NewScanner(r)
	i := 0
	for scanner.Scan() {
		i++
	}
	return &jsonl{
		file:   r,
		fileMu: &sync.Mutex{},
		len:    i,
		lenMu:  &sync.Mutex{},
	}, nil
}

var _ io.WriteCloser = &jsonl{}

// jsonl implements a jsonl io.WriteCloser
type jsonl struct {
	name   string
	file   io.ReadWriteCloser
	fileMu *sync.Mutex
	len    int
	lenMu  *sync.Mutex
}

// Write appends entries to a jsonl file.
// Multiple entries can be processed at once, however writes
// are all-or-nothing. If any of the entries fails to decode as
// valid JSON, none of the byte slice will be written.
func (j *jsonl) Write(p []byte) (n int, err error) {
	// jsonl specifies that all input must be utf8.
	if !utf8.Valid(p) {
		return 0, ErrNotUTF8
	}
	// jsonl specifies that each line is valid JSON,
	// and that the line separator is '\n'.
	entries := bytes.Split(p, []byte{'\n'})
	for _, entry := range entries {
		// If any of the entries fail to decode, bail the entire operation.
		if !json.Valid(entry) {
			return 0, ErrNotJSON
		}
	}
	// If the last entry doesn't have a newline, add it.
	if p[len(p)-1] != '\n' {
		p = append(p, '\n')
	}
	// Write
	j.fileMu.Lock()
	wrote, err := j.file.Write(p)
	j.fileMu.Unlock()
	if err != nil {
		return wrote, err
	}
	// update len
	j.lenMu.Lock()
	defer j.lenMu.Unlock()
	j.len += bytes.Count(p, []byte{'\n'})
	return wrote, nil
}

// Close the jsonl file
func (j *jsonl) Close() error {
	return j.file.Close()
}

// Len returns the number of entries in the jsonl file.
func (j *jsonl) Len() int {
	return j.len
}

// BytesAt returns the bytes from the jsonl file at the specified line
func (j *jsonl) BytesAt(line int) ([]byte, error) {
	if line < 1 {
		return nil, fmt.Errorf("line/entry number cannot be less than one")
	}
	f, err := os.Open(j.name) // manual seeking is... hard. This is easy.
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	i := 0
	for scanner.Scan() {
		i++
		if i == line {
			return scanner.Bytes(), nil
		}
	}
	if scanner.Err() != nil {
		return nil, scanner.Err()
	}
	return nil, ErrEntryNotFound
}

// At returns the jsonl entry at a given position marshaled to v.
func (j *jsonl) At(line int, v interface{}) error {
	dat, err := j.BytesAt(line)
	if err != nil {
		return err
	}
	return json.Unmarshal(dat, v)
}

// Latest marshals the latest non-corrupt item written to the jsonl file to v.
func (j *jsonl) Latest(v interface{}) error {
	err := j.At(j.Len(), v)
	if err != nil && err.Error() == "unexpected end of JSON input" {
		l := j.Len() - 1
		if l < 1 {
			return fmt.Errorf("first and only entry was corrupt. Wrong file?")
		}
		return j.At(l, v)
	}
	return nil
}

// Add marshals and writes a JSON object
func (j *jsonl) Add(v interface{}) error {
	dat, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = j.Write(dat)
	return err
}
