package pipeline

import (
	"errors"
	"net/http"

	"github.com/niclasedge/tollgate/internal/jsonsplice"
)

// Anthropic error types used by the pipeline (ADR-0004, ADR-0009).
const (
	ErrTypeInvalidRequest = "invalid_request_error"
	ErrTypeAuthentication = "authentication_error"
	ErrTypeAPI            = "api_error"
)

// StageError aborts the pipeline. Message is a fixed, plaintext-free text for
// the client; Cause is for internal logging only and is not part of Error().
type StageError struct {
	Stage   string
	Status  int
	Type    string
	Message string
	Cause   error
}

func (e *StageError) Error() string { return "pipeline: stage " + e.Stage + ": " + e.Message }
func (e *StageError) Unwrap() error { return e.Cause }

// asStageError converts any stage error into a *StageError; unknown errors
// become 500 api_error with a generic message so no stage-internal text
// (which could contain plaintext) reaches the client.
func asStageError(stage string, err error) *StageError {
	var se *StageError
	if errors.As(err, &se) {
		if se.Stage == "" {
			cp := *se
			cp.Stage = stage
			return &cp
		}
		return se
	}
	return &StageError{Stage: stage, Status: http.StatusInternalServerError, Type: ErrTypeAPI,
		Message: "internal error in stage " + stage, Cause: err}
}

// ErrorResponse returns the HTTP status and the Anthropic-format JSON body
// for err. Output is byte-stable.
func ErrorResponse(err error) (int, []byte) {
	se := asStageError("", err)
	b := []byte(`{"type":"error","error":{"type":"`)
	b = jsonsplice.Escape(b, se.Type)
	b = append(b, `","message":"`...)
	b = jsonsplice.Escape(b, se.Message)
	b = append(b, `"}}`...)
	return se.Status, b
}
