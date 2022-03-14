package jsonl

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestBasicUsage(t *testing.T) {
	// Imagine your device is an embedded device but you
	// want to make confiuration changes. If you, to quote a phrase,
	// monkey-patch a JSON config, what happens if power is lost
	// after your first change, but before your last? App logic bug.
	// What happens if power is lost during the write of the config
	// file itself?
	//
	// In these cases, the sacraficed disk storage is well worth the
	// ability to recover from a previous value.
	testDir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(testDir, "example.jsonl")

	// Some struct or data store we want to save.
	type Config struct {
		Key string `json:"key"`
	}

	// Open the jsonl file, creating it if it doesn't exist.
	jstore, err := File(filename)
	if err != nil {
		t.Fatal(err)
	}

	// config is what we want to save
	config := &Config{Key: "value"}
	// Add config to the jsonl store
	err = jstore.Add(config)
	if err != nil {
		t.Fatal(err)
	}

	// Retrieve the latest config
	latest := &Config{}
	err = jstore.Latest(latest)
	if err != nil {
		t.Fatal(err)
	}

	// The values should match
	if latest.Key != config.Key {
		t.Fatal("values don't match!")
	}

	// But imagine there was some horrible event and there was corruption.
	// We can simulate this by writing garbage to the underlying io.ReadWriteCloser
	_, err = jstore.file.Write([]byte(`{ "maybe": {"this was once valid json, but it isn't anymore`))
	if err != nil {
		panic(err)
	}

	// Simulating a power loss event, we would have to re-open the jstore
	_ = jstore.Close()
	jstore, err = File(filename)
	if err != nil {
		t.Fatal(err)
	}

	// Now when we try to retrieve the latest item, it's garbage!
	latest = &Config{}
	// But Latest() handles this for us: it returns the prior non-corrupt item.
	err = jstore.Latest(latest)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Key != config.Key {
		t.Fatal("values don't match!")
	}

}

func TestJsonl(t *testing.T) {
	testDir, err := os.MkdirTemp("", "")
	if err != nil {
		panic(err)
	}
	filename := filepath.Join(testDir, "test.jsonl")
	j, err := File(filename)
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
	j, err = File(filename)
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
	if err := j.Latest(&SomeStruct); err != nil {
		t.Fatalf("error getting Latest jsonl entry: %s", err.Error())
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
