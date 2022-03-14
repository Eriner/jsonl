package jsonl

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestJsonl(t *testing.T) {
	testDir, err := os.MkdirTemp("", "")
	if err != nil {
		panic(err)
	}
	filename := filepath.Join(testDir, "test.jsonl")
	j, err := Open(filename)
	if err != nil {
		t.Fatalf("unable to open jsonl: %s", err.Error())
	}
	if j.Len() != 0 {
		t.Fatal("new jsonl file has non-zero length")
	}
	const jsonString = `{"abc":{ "key":"value"}}`
	wrote, err := j.Write([]byte(jsonString))
	if err != nil {
		t.Fatalf("unable to write json to jsonl: %s", err.Error())
	}
	if wrote != len(jsonString)+1 {
		t.Fatal("did not write len(input) + 1 byte (newline)")
	}
	const invalidJsonString = `aaaa`
	_, err = j.Write([]byte(invalidJsonString))
	if err != ErrNotJSON {
		t.Fatalf("expecting ErrNotJSON, got %q", err.Error())
	}
	if j.Len() != 1 {
		t.Fatal("unexpected jsonl length: expected one entry")
	}
	_, _ = j.Write([]byte(jsonString))
	if j.Len() != 2 {
		t.Fatal("unexpected jsonl length: expected two entries")
	}
	_, err = j.Write([]byte(`{"abc":{"k":"v"}}
	{"efg":{"h":"i"}}`))
	if err != nil {
		t.Fatalf("error writing multiple json entries: %s", err.Error())
	}
	if j.Len() != 4 {
		t.Fatal("unexpected jsonl length: expected four entries")
	}
	if err := j.Close(); err != nil {
		t.Fatalf("shouldn't error out on close: %s", err.Error())
	}
	j = nil
	j, err = Open(filename)
	if err != nil {
		t.Fatalf("unable to re-open jsonl: %s", err.Error())
	}
	defer j.Close()
	if j.Len() != 4 {
		t.Fatalf("unexpected jsonl length: expected those same four entries, got %d", j.Len())
	}
	var SomeStruct = struct {
		A struct {
			Key string `json:"key"`
		} `json:"abc"`
	}{
		A: struct {
			Key string `json:"key"`
		}{
			Key: "abc",
		},
	}
	dat, _ := json.Marshal(SomeStruct)
	_, err = j.Write(dat)
	if err != nil {
		t.Fatalf("error writing marshaled json shruct")
	}
	SomeStruct.A.Key = "xxxxxxxxxxxx"
	if err := j.Last(&SomeStruct); err != nil {
		t.Fatalf("error getting Last jsonl entry: %s", err.Error())
	}
	dat2, _ := json.Marshal(SomeStruct)
	if !bytes.Equal(dat, dat2) {
		t.Fatal("write and read of struct to jsonl returned different results")
	}
	if err := j.At(99, &SomeStruct); !errors.Is(err, ErrEntryNotFound) {
		t.Fatal("invalid At(id) did not return ErrEntryNotFound")
	}
	if err := j.Close(); err != nil {
		t.Fatalf("non-nil err on Close(): %s", err.Error())
	}
	if _, err = j.Write(dat); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("something didn't close right")
	}

}
