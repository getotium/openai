package openai

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseRequests_Valid(t *testing.T) {
	t.Parallel()
	in := strings.Join([]string{
		`{"custom_id":"a","method":"POST","url":"/v1/chat/completions","body":{"model":"qwen3-8b","messages":[{"role":"user","content":"hi"}]}}`,
		``, // blank lines are skipped
		`{"custom_id":"b","method":"POST","url":"/v1/chat/completions","body":{"model":"gemma-2-9b"}}`,
		``, // trailing newline
	}, "\n")

	reqs, errs := ParseRequests(strings.NewReader(in), EndpointChatCompletions)
	if len(errs) != 0 {
		t.Fatalf("unexpected line errors: %v", errs)
	}
	if len(reqs) != 2 {
		t.Fatalf("got %d requests, want 2", len(reqs))
	}
	if reqs[0].CustomID != "a" || reqs[0].Model != "qwen3-8b" || reqs[0].Seq != 1 {
		t.Fatalf("req[0] = %+v", reqs[0])
	}
	if reqs[1].CustomID != "b" || reqs[1].Model != "gemma-2-9b" || reqs[1].Seq != 3 {
		t.Fatalf("req[1] = %+v", reqs[1])
	}
	// Body is preserved byte-for-byte (modulo whitespace) — no field dropping.
	if !json.Valid(reqs[0].Body) || !bytes.Contains(reqs[0].Body, []byte("messages")) {
		t.Fatalf("body not preserved: %s", reqs[0].Body)
	}
}

func TestParseRequests_LineErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, line, wantSubstr string
	}{
		{"bad json", `{"custom_id":"a" "body":{}}`, "invalid JSON"},
		{"missing custom_id", `{"method":"POST","url":"/v1/chat/completions","body":{"model":"m"}}`, "missing custom_id"},
		{"bad method", `{"custom_id":"a","method":"GET","url":"/v1/chat/completions","body":{"model":"m"}}`, "unsupported method"},
		{"url mismatch", `{"custom_id":"a","method":"POST","url":"/v1/embeddings","body":{"model":"m"}}`, "does not match"},
		{"missing body", `{"custom_id":"a","method":"POST","url":"/v1/chat/completions"}`, "missing body"},
		{"missing model", `{"custom_id":"a","method":"POST","url":"/v1/chat/completions","body":{"messages":[]}}`, "missing model"},
		{"unknown field", `{"custom_id":"a","method":"POST","url":"/v1/chat/completions","body":{"model":"m"},"extra":1}`, "invalid JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reqs, errs := ParseRequests(strings.NewReader(tc.line), EndpointChatCompletions)
			if len(reqs) != 0 {
				t.Fatalf("got %d requests, want 0", len(reqs))
			}
			if len(errs) != 1 {
				t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
			}
			if !strings.Contains(errs[0].Message, tc.wantSubstr) {
				t.Fatalf("error %q does not contain %q", errs[0].Message, tc.wantSubstr)
			}
			if errs[0].Line != 1 {
				t.Fatalf("error line = %d, want 1", errs[0].Line)
			}
		})
	}
}

func TestParseRequests_DuplicateCustomID(t *testing.T) {
	t.Parallel()
	in := strings.Join([]string{
		`{"custom_id":"dup","method":"POST","url":"/v1/chat/completions","body":{"model":"m"}}`,
		`{"custom_id":"dup","method":"POST","url":"/v1/chat/completions","body":{"model":"m"}}`,
	}, "\n")
	reqs, errs := ParseRequests(strings.NewReader(in), EndpointChatCompletions)
	if len(reqs) != 1 {
		t.Fatalf("got %d requests, want 1 (the first)", len(reqs))
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "duplicate custom_id") {
		t.Fatalf("want one duplicate error, got %v", errs)
	}
}

func TestParseRequests_DefaultMethodAllowed(t *testing.T) {
	t.Parallel()
	// OpenAI's spec shows method as part of the line, but a missing method is treated
	// as the implied POST rather than rejected.
	in := `{"custom_id":"a","url":"/v1/chat/completions","body":{"model":"m"}}`
	reqs, errs := ParseRequests(strings.NewReader(in), EndpointChatCompletions)
	if len(errs) != 0 || len(reqs) != 1 {
		t.Fatalf("reqs=%d errs=%v", len(reqs), errs)
	}
}

func TestWriteOutputs_RoundTrip(t *testing.T) {
	t.Parallel()
	lines := []BatchRequestOutput{
		{ID: "batch_req_1", CustomID: "a", Response: &OutputResponse{StatusCode: 200, RequestID: "r1", Body: json.RawMessage(`{"ok":true}`)}},
		{ID: "batch_req_2", CustomID: "b", Error: &OutputError{Code: "run_failed", Message: "boom"}},
	}
	var buf bytes.Buffer
	if err := WriteOutputs(&buf, lines); err != nil {
		t.Fatalf("WriteOutputs: %v", err)
	}
	got := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(got), buf.String())
	}
	var first BatchRequestOutput
	if err := json.Unmarshal([]byte(got[0]), &first); err != nil {
		t.Fatalf("unmarshal line 0: %v", err)
	}
	if first.CustomID != "a" || first.Response == nil || first.Response.StatusCode != 200 {
		t.Fatalf("line 0 round-trip mismatch: %+v", first)
	}
	// A failed line carries error, not response.
	if !strings.Contains(got[1], `"response":null`) || !strings.Contains(got[1], "run_failed") {
		t.Fatalf("line 1 = %s", got[1])
	}
}
