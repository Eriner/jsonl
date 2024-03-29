package jsonl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func Example_main() {
	// Imagine your device is an embedded device but you
	// want to make confiuration changes. If you, to quote a phrase,
	// monkey-patch a JSON config, what happens if power is lost
	// after your first change, but before your last? App logic bug.
	// What happens if power is lost during the write of the config
	// file itself?
	//
	// JSONL provides versioned, write-error recoverable JSON data-stores.
	filename := "test.jsonl"

	// Some struct or data store we want to save.
	type Config struct {
		Key string `json:"key"`
	}

	// Open the jsonl file, creating it if it doesn't exist.
	store, err := OpenFile(filename)
	if err != nil {
		panic(err)
	}
	defer store.Close()

	// create readers and writers for the jsonl data store
	reader := json.NewDecoder(store)
	writer := json.NewEncoder(store)

	// config is what we want to save
	config := Config{Key: "value"}

	// Add config to the jsonl store
	if err := writer.Encode(&config); err != nil {
		panic(err)
	}

	// Retrieve the latest config
	latest := Config{}
	if err := reader.Decode(&latest); err != nil {
		panic(err)
	}

	// The values should match
	if latest.Key != config.Key {
		panic(err)
	}
}

func TestMidWriteFailure(t *testing.T) {
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
	filename := filepath.Join(testDir, "test_failure.jsonl")

	// Some struct or data store we want to save.
	type Config struct {
		Key string `json:"key"`
	}

	// Open the jsonl file, creating it if it doesn't exist.
	store, err := OpenFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// create readers and writers for the jsonl data store
	reader := json.NewDecoder(store)
	writer := json.NewEncoder(store)

	// config is what we want to save
	config := Config{Key: "value"}

	// Add config to the jsonl store
	if err := writer.Encode(&config); err != nil {
		t.Fatal(err)
	}

	// Retrieve the latest config
	latest := Config{}
	if err := reader.Decode(&latest); err != nil {
		t.Fatal(err)
	}

	// The values should match
	if latest.Key != config.Key {
		t.Fatal("values don't match!")
	}

	// But imagine there was some horrible event and there was corruption in the middle of a write.
	// We can simulate this by writing garbage to the underlying os.File:
	_, err = store.f.Write([]byte(`{ "maybe": {"this was once valid json, but it isn't anymore`))
	if err != nil {
		panic(err)
	}
	if err := store.f.Sync(); err != nil {
		panic(err)
	}

	// Simulating a power loss event, we would have to re-open the jstore
	//_ = store.Close()
	store = nil

	store, err = OpenFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	reader = json.NewDecoder(store)

	// Now when we try to retrieve the latest item, it'll be garbage!
	// But the jsonl Read() method handles this for us: it returns the
	// last non-corrupt item, which should be the first write we performed
	// above.
	latest = Config{}
	if err := reader.Decode(&latest); err != nil {
		t.Fatal(err)
	}
	if latest.Key != config.Key {
		t.Fatal("values don't match!")
	}
}

func TestExpectedRead(t *testing.T) {
	testDir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(testDir, "validity.jsonl")
	type Entry struct {
		V int `json:"number"`
	}
	store, err := OpenFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	reader := json.NewDecoder(store)
	writer := json.NewEncoder(store)

	for i := 0; i <= 12; i++ {
		if err := writer.Encode(&Entry{V: i}); err != nil {
			t.Fatal(err)
		}
	}
	latest := &Entry{}
	if err := reader.Decode(latest); err != nil {
		t.Fatal(err)
	}
	if latest.V != 12 {
		t.Fatalf("got wrong entry when performing reader.Decode(). Expected (%d), got (%d)\n", 12, latest.V)
	}
}

func TestEmptyDecode(t *testing.T) {
	testDir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(testDir, "empty.jsonl")
	type Entry struct {
		V int `json:"number"`
	}
	store, err := OpenFile(filename)
	defer store.Close()

	ch := make(chan struct{})
	go func() {
		null := &Entry{}
		if err := store.Decode(null); err != nil {
			t.Fatal(err)
		}
		ch <- struct{}{}
	}()

	select {
	case <-time.After(1 * time.Second):
		t.Fatal("failed to return error from Read() on empty file")
	case <-ch:

	}
}
