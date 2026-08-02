# openai

The **wire types** for an OpenAI-compatible **Files + Batch** API, in Go — the exact JSON shapes a
base-URL-swapped OpenAI client expects to send and receive, plus a codec for the batch request/
response `.jsonl` files.

> This is the **server-side contract**, not an OpenAI client. If you're building a service that a
> stock OpenAI SDK can talk to by just changing the base URL, these are the types your endpoints
> marshal — and the conformance layer that keeps you honest against the upstream spec.

- **Dependency-free.** Stdlib only. It models the external contract and nothing else — no domain
  logic, no storage, no opinions.
- **Files + Batch surface.** `FileObject`, `Batch`, `RequestCounts`, `ListResponse[T]`, the batch
  error shapes, and the per-line `BatchRequestInput` / `BatchRequestOutput` records.
- **Batch file codec.** `ParseRequests` reads an uploaded batch input `.jsonl`; `WriteOutputs`
  writes the generated output `.jsonl` — the two ends of the async batch flow.
- **OpenAI-shaped errors.** `APIError` / `NewInvalidRequest` produce the `{"error": {...}}` body
  clients parse.

## Install

```
go get github.com/getotium/openai
```

## Usage

```go
// Parse an uploaded batch input file (one BatchRequestInput per line).
reqs, lineErrs := openai.ParseRequests(inputFile, "/v1/chat/completions")

// ... run each request through your engine ...

// Write the batch output file the client will download.
err := openai.WriteOutputs(outputFile, outputs) // []openai.BatchRequestOutput

// Marshal a Batch object / a paginated list exactly as an OpenAI client expects.
body := openai.NewList([]openai.FileObject{f1, f2})
```

## Provenance

Extracted from [Otium](https://getotium.ai), whose public API is OpenAI-compatible so customers
integrate by pointing an existing OpenAI client at it. These are the shapes that make "just change
the base URL" true; the reference server and conformance suite that exercise them are being published
alongside.

## License

[Apache-2.0](LICENSE).
