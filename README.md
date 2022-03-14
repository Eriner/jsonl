# jsonl

A Go package to provide [`jsonl`](https://jsonlines.org/) support.

Package is WIP and unstable.

## Usage

```go
// JSON to be stored
type Config struct {
	Key string `json:"key"`
}
c := &Config{Key: "value"}

// Add it
configs, err := jsonl.File("config.jsonl")
if err != nil {
	panic(err)
}
if err := configs.Add(c); err != nil {
	panic(err)
}

// Retrieve it
latest, err := configs.Latest()
if err != nil {
	panic(err)
}
log.Printf("%+v", latest)
```
