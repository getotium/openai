// Package openai holds Otium's OpenAI-compatible wire types: the JSON shapes for the
// Files and Batch APIs that a base-URL-swapped OpenAI client expects to see. It is the
// conformance layer for the public surface — the first customer (AR15.Build) integrates
// by pointing its existing OpenAI client at Otium, so these shapes must match OpenAI's
// exactly where the client depends on them.
//
// Like pkg/huggingface, this package is deliberately dependency-free and free of Otium
// imports: it models the external contract only (objects + a JSONL codec for batch
// request/response files), returning plain wire types. The mapping between these and
// Otium's domain entities (pkg/batch, pkg/job) lives in the surface layer (pkg/api), not
// here, so this package stays lift-ready and easy to reason about against the upstream
// spec.
package openai

import "encoding/json"

// Object-type discriminators OpenAI stamps on each resource.
const (
	ObjectFile  = "file"
	ObjectBatch = "batch"
	ObjectList  = "list"
)

// Batch statuses, matching OpenAI's batch lifecycle exactly. Otium's internal state
// machine (pkg/batch) uses the same names so the wire mapping is identity.
const (
	BatchValidating = "validating"
	BatchFailed     = "failed"
	BatchInProgress = "in_progress"
	BatchFinalizing = "finalizing"
	BatchCompleted  = "completed"
	BatchExpired    = "expired"
	BatchCancelling = "cancelling"
	BatchCancelled  = "cancelled"
)

// File purposes. Otium only accepts batch input; "batch_output" is what it stamps on
// the files it generates (results and errors), matching OpenAI.
const (
	PurposeBatch       = "batch"
	PurposeBatchOutput = "batch_output"
)

// EndpointChatCompletions is the only batch endpoint Otium supports today — the surface
// AR15.Build's enrichment pipeline uses. OpenAI also allows /v1/embeddings and
// /v1/completions; those are additive later.
const EndpointChatCompletions = "/v1/chat/completions"

// CompletionWindow24h is the only completion window OpenAI's Batch API documents. Otium
// accepts it (and maps it to an SLA tier) plus its own tier names as an extension.
const CompletionWindow24h = "24h"

// FileObject is the OpenAI representation of an uploaded or generated file. Timestamps
// are Unix seconds, as OpenAI emits them.
type FileObject struct {
	ID        string `json:"id"`
	Object    string `json:"object"` // always "file"
	Bytes     int64  `json:"bytes"`
	CreatedAt int64  `json:"created_at"`
	Filename  string `json:"filename"`
	Purpose   string `json:"purpose"`
}

// Batch is the OpenAI representation of a batch. Optional timestamps are pointers so an
// unset one serializes to JSON null, exactly as OpenAI emits them; Errors and Metadata
// likewise serialize to null/absent when empty.
type Batch struct {
	ID               string            `json:"id"`
	Object           string            `json:"object"` // always "batch"
	Endpoint         string            `json:"endpoint"`
	Errors           *ErrorList        `json:"errors"`
	InputFileID      string            `json:"input_file_id"`
	CompletionWindow string            `json:"completion_window"`
	Status           string            `json:"status"`
	OutputFileID     string            `json:"output_file_id,omitempty"`
	ErrorFileID      string            `json:"error_file_id,omitempty"`
	CreatedAt        int64             `json:"created_at"`
	InProgressAt     *int64            `json:"in_progress_at"`
	ExpiresAt        *int64            `json:"expires_at"`
	FinalizingAt     *int64            `json:"finalizing_at"`
	CompletedAt      *int64            `json:"completed_at"`
	FailedAt         *int64            `json:"failed_at"`
	ExpiredAt        *int64            `json:"expired_at"`
	CancellingAt     *int64            `json:"cancelling_at"`
	CancelledAt      *int64            `json:"cancelled_at"`
	RequestCounts    RequestCounts     `json:"request_counts"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// RequestCounts is the per-batch progress tally OpenAI reports.
type RequestCounts struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// ErrorList is the non-fatal-error envelope on a batch (e.g. malformed input lines
// discovered during validation).
type ErrorList struct {
	Object string       `json:"object"` // always "list"
	Data   []BatchError `json:"data"`
}

// BatchError is one entry in a batch's error list. Line is 1-based and points at the
// offending input-file line when applicable.
type BatchError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param,omitempty"`
	Line    *int   `json:"line"`
}

// ListResponse is the generic OpenAI list envelope used by GET /v1/files and
// GET /v1/batches. HasMore/cursors are omitted — Otium returns a single page for the
// PoC.
type ListResponse[T any] struct {
	Object  string `json:"object"` // always "list"
	Data    []T    `json:"data"`
	HasMore bool   `json:"has_more"`
}

// NewList wraps a slice in the OpenAI list envelope. A nil slice serializes as an empty
// array, not null, matching OpenAI.
func NewList[T any](data []T) ListResponse[T] {
	if data == nil {
		data = []T{}
	}
	return ListResponse[T]{Object: ObjectList, Data: data}
}

// BatchRequestInput is one line of an uploaded batch input file (JSONL). Body is kept
// raw so the request is passed to the runtime byte-for-byte — Otium does not re-model
// the chat-completion request and so cannot silently drop fields the client depends on.
type BatchRequestInput struct {
	CustomID string          `json:"custom_id"`
	Method   string          `json:"method"`
	URL      string          `json:"url"`
	Body     json.RawMessage `json:"body"`
}

// BatchRequestOutput is one line of a generated batch output (or error) file (JSONL).
// Exactly one of Response / Error is set per line, matching OpenAI.
type BatchRequestOutput struct {
	ID       string          `json:"id"`
	CustomID string          `json:"custom_id"`
	Response *OutputResponse `json:"response"`
	Error    *OutputError    `json:"error"`
}

// OutputResponse wraps the per-request HTTP result inside an output line. Body is the
// raw chat-completion response the runtime produced.
type OutputResponse struct {
	StatusCode int             `json:"status_code"`
	RequestID  string          `json:"request_id"`
	Body       json.RawMessage `json:"body"`
}

// OutputError is the per-request error inside an output line (and the sole content of
// the error file's lines).
type OutputError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
