# jsonl

A Go package to provide [`jsonl`](https://jsonlines.org/) support.

[![Go Reference](https://pkg.go.dev/badge/github.com/eriner/jsonl.svg)](https://pkg.go.dev/github.com/eriner/jsonl)

Package is WIP and unstable.

## About

JSON Lines is very useful when creating power-loss resistant applications.
Without JSON Lines, you risk a power loss corrupting half of a write.

From my memory the Ubiquiti CloudKey Gen 1, lacking a battery, had a similar problem:
configuration/database writes could corrupt the device such that it would not boot.

The use of JSONL as opposed to regular JSON solves this problem.

## Usage

```go
store, err := jsonl.OpenFile("config.jsonl")
if err != nil {
	panic(err)
}
defer store.Close()

reader := json.NewDecoder(store)
writer := json.NewEncoder(store)

data := struct{
	Key string
}{
	Key: "value",
}

if err := writer.Encode(&data); err != nil {
	panic(err)
}
if err := reader.Decode(&data); err != nil {
	panic(err)
}
log.Printf("%+v\n", data)
```
