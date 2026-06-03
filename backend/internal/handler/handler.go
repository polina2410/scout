package handler

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/polina2410/scout/backend/internal/middleware"
)

const (
	ErrCodeBadRequest          = "BAD_REQUEST"           // 400
	ErrCodeUnauthorized        = "UNAUTHORIZED"          // 401
	ErrCodeNotFound            = "NOT_FOUND"             // 404
	ErrCodeConflict            = "CONFLICT"              // 409
	ErrCodeUnprocessableEntity = "UNPROCESSABLE_ENTITY"  // 422
	ErrCodeServiceUnavailable  = "SERVICE_UNAVAILABLE"   // 503
	ErrCodeInternal            = "INTERNAL_ERROR"        // 500
)

// ErrorResponse is the JSON body returned for all non-2xx responses.
type ErrorResponse struct {
	RequestID string `json:"request_id"`
	Message   string `json:"message"`
	Code      string `json:"code"`
}

// WriteError writes a JSON error response.
// status is the HTTP status code. code is one of the named Err* constants.
// message is a safe, human-readable description.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code string, message string) {
	resp := ErrorResponse{
		RequestID: middleware.RequestIDFromContext(r.Context()),
		Message:   message,
		Code:      code,
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(resp); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Write error is intentionally ignored: the status line is already committed,
	// so there is nothing meaningful to do if the body write fails.
	buf.WriteTo(w) //nolint:errcheck
}

// WriteJSON writes a 2xx JSON response.
// status is the HTTP status code. v is JSON-encoded into the body.
func WriteJSON[T any](w http.ResponseWriter, status int, v T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
