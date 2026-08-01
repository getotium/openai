package openai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// MaxLineBytes bounds a single JSONL line so one pathological line can't exhaust memory
// while parsing an input file. OpenAI's per-request body limit is well under this.
const MaxLineBytes = 4 << 20 // 4 MiB

// ParsedRequest is one validated input line: the raw line plus the model pulled out of
// its body (the one field Otium must read to route the request to a worker). Seq is the
// 1-based line number, used for stable custom-id fallbacks and error reporting.
type ParsedRequest struct {
	Seq      int
	CustomID string
	Model    string
	Body     json.RawMessage
}

// LineError reports a malformed input line. Line is 1-based.
type LineError struct {
	Line    int
	Message string
}

func (e LineError) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Message) }

// bodyModel is the only field ParseRequests reads out of a request body.
type bodyModel struct {
	Model string `json:"model"`
}

// ParseRequests reads a batch input file (JSONL) and validates each line against the
// constraints Otium enforces: well-formed JSON, a supported method+url, a non-empty
// body carrying a model, and unique custom_ids. Blank lines are skipped (trailing
// newlines are common). It returns the parsed requests and a slice of per-line errors;
// a caller treats any line error as a validation failure for the whole batch (matching
// OpenAI, which fails a batch whose input doesn't validate).
//
// endpoint is the batch's declared endpoint; every line's url must match it.
func ParseRequests(r io.Reader, endpoint string) ([]ParsedRequest, []LineError) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), MaxLineBytes)

	var out []ParsedRequest
	var errs []LineError
	seen := make(map[string]bool)
	line := 0
	for sc.Scan() {
		line++
		raw := bytes.TrimSpace(sc.Bytes())
		if len(raw) == 0 {
			continue
		}
		var in BatchRequestInput
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			errs = append(errs, LineError{Line: line, Message: "invalid JSON: " + cleanJSONErr(err)})
			continue
		}
		if in.CustomID == "" {
			errs = append(errs, LineError{Line: line, Message: "missing custom_id"})
			continue
		}
		if seen[in.CustomID] {
			errs = append(errs, LineError{Line: line, Message: "duplicate custom_id " + in.CustomID})
			continue
		}
		seen[in.CustomID] = true
		if in.Method != "" && !strings.EqualFold(in.Method, "POST") {
			errs = append(errs, LineError{Line: line, Message: "unsupported method " + in.Method + " (only POST)"})
			continue
		}
		if in.URL != endpoint {
			errs = append(errs, LineError{Line: line, Message: fmt.Sprintf("url %q does not match batch endpoint %q", in.URL, endpoint)})
			continue
		}
		if len(in.Body) == 0 {
			errs = append(errs, LineError{Line: line, Message: "missing body"})
			continue
		}
		var bm bodyModel
		if err := json.Unmarshal(in.Body, &bm); err != nil {
			errs = append(errs, LineError{Line: line, Message: "invalid body: " + cleanJSONErr(err)})
			continue
		}
		if bm.Model == "" {
			errs = append(errs, LineError{Line: line, Message: "body is missing model"})
			continue
		}
		out = append(out, ParsedRequest{Seq: line, CustomID: in.CustomID, Model: bm.Model, Body: in.Body})
	}
	if err := sc.Err(); err != nil {
		errs = append(errs, LineError{Line: line + 1, Message: "read error: " + err.Error()})
	}
	return out, errs
}

// WriteOutputs encodes output lines as JSONL (one compact JSON object per line). It is
// the inverse of ParseRequests: the result and error files are written this way.
func WriteOutputs(w io.Writer, lines []BatchRequestOutput) error {
	bw := bufio.NewWriter(w)
	enc := json.NewEncoder(bw)
	for _, l := range lines {
		if err := enc.Encode(l); err != nil { // Encode appends a newline
			return fmt.Errorf("openai: write output line: %w", err)
		}
	}
	return bw.Flush()
}

// cleanJSONErr trims the noisy prefix off a json error for a friendlier message.
func cleanJSONErr(err error) string {
	return strings.TrimPrefix(err.Error(), "json: ")
}
