# jsonl

A WIP Go package to provide [`jsonl`](https://jsonlines.org/) support.

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
configs.Add(c)

// Retrieve it
latest, err := configs.Latest()
if err != nil {
	panic(err)
}
log.Printf("%+v", latest)
```

