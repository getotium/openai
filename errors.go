package openai

// Error type discriminators OpenAI uses in its error envelope.
const (
	ErrorTypeInvalidRequest = "invalid_request_error"
	ErrorTypeNotFound       = "invalid_request_error" // OpenAI reports 404s as invalid_request_error
	ErrorTypeServer         = "server_error"
)

// APIError is the body of an OpenAI error response: {"error": {...}}. A base-URL-swapped
// client unwraps the inner object, so the shape must match. Param and Code are pointers
// so they serialize to null when unset, as OpenAI emits them.
type APIError struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody is the inner error object.
type ErrorBody struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    *string `json:"code"`
}

// NewError builds an error envelope with the given message and type.
func NewError(message, errType string) APIError {
	return APIError{Error: ErrorBody{Message: message, Type: errType}}
}

// NewInvalidRequest is the common case: a 4xx invalid-request error.
func NewInvalidRequest(message string) APIError {
	return NewError(message, ErrorTypeInvalidRequest)
}
