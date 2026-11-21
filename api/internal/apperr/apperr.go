// Package apperr defines typed application errors that map to HTTP responses.
// Every API error is returned as: { "error": { "code": "...", "message": "..." } }
package apperr

import "net/http"

// Error is a client-facing error with an HTTP status and a stable code.
type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func BadRequest(msg string) *Error   { return New(http.StatusBadRequest, "BAD_REQUEST", msg) }
func Unauthorized(msg string) *Error { return New(http.StatusUnauthorized, "UNAUTHORIZED", msg) }
func Forbidden(msg string) *Error    { return New(http.StatusForbidden, "FORBIDDEN", msg) }
func NotFound(msg string) *Error     { return New(http.StatusNotFound, "NOT_FOUND", msg) }
func Conflict(msg string) *Error     { return New(http.StatusConflict, "CONFLICT", msg) }
func Validation(msg string) *Error {
	return New(http.StatusUnprocessableEntity, "VALIDATION_ERROR", msg)
}
func InvalidTransition(msg string) *Error {
	return New(http.StatusConflict, "INVALID_TRANSITION", msg)
}
