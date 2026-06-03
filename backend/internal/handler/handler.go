package handler

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/polina2410/scout/backend/internal/middleware"
)

// Error codes match the openapi.yaml enum values exactly.
// Codes for statuses not in the openapi contract (409, 422, 503) use descriptive strings.
const (
	ErrCodeValidation          = "ValidationError"        // 400 — matches openapi.yaml ValidationError.code enum
	ErrCodeUnauthorized        = "AuthenticationRequired"  // 401 — matches openapi.yaml AuthenticationError.code enum
	ErrCodeNotFound            = "NotFound"                // 404 — matches openapi.yaml NotFoundError.code enum
	ErrCodeConflict            = "Conflict"                // 409 — not in openapi contract
	ErrCodeUnprocessableEntity = "UnprocessableEntity"     // 422 — not in openapi contract
	ErrCodeTooManyRequests     = "TooManyRequests"         // 429 — not in openapi contract
	ErrCodeInternal            = "InternalServerError"     // 500 — matches openapi.yaml InternalServerError.code enum
	ErrCodeServiceUnavailable  = "ServiceUnavailable"      // 503 — not in openapi contract (thumbnail semaphore)
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

// HealthResponse is the JSON body returned by GET /health.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// ValidationDetail is one entry in the details array of a ValidationError response.
type ValidationDetail struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

type validationErrorResponse struct {
	RequestID string             `json:"request_id"`
	Message   string             `json:"message"`
	Code      string             `json:"code"`
	Details   []ValidationDetail `json:"details"`
}

// WriteValidationError writes a 400 response with per-field detail entries.
func WriteValidationError(w http.ResponseWriter, r *http.Request, details []ValidationDetail) {
	resp := validationErrorResponse{
		RequestID: middleware.RequestIDFromContext(r.Context()),
		Message:   "request validation failed",
		Code:      ErrCodeValidation,
		Details:   details,
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(resp); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	buf.WriteTo(w) //nolint:errcheck
}

type notFoundErrorResponse struct {
	RequestID  string `json:"request_id"`
	Message    string `json:"message"`
	Code       string `json:"code"`
	ResourceID string `json:"resource_id"`
}

// WriteNotFoundError writes a 404 response with the resource_id that was not found.
func WriteNotFoundError(w http.ResponseWriter, r *http.Request, resourceID string) {
	resp := notFoundErrorResponse{
		RequestID:  middleware.RequestIDFromContext(r.Context()),
		Message:    "resource not found",
		Code:       ErrCodeNotFound,
		ResourceID: resourceID,
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(resp); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	buf.WriteTo(w) //nolint:errcheck
}
