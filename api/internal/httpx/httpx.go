// Package httpx contains small helpers for JSON requests and responses.
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"ticketing/api/internal/apperr"
)

const maxBodyBytes = 1 << 20 // 1 MB

type errorBody struct {
	Error *apperr.Error `json:"error"`
}

// WriteJSON writes v as a JSON response with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		if err := json.NewEncoder(w).Encode(v); err != nil {
			slog.Error("failed to encode response", "err", err)
		}
	}
}

// WriteError converts any error into the standard error response.
// Unknown errors are logged and hidden behind a generic 500 message.
func WriteError(w http.ResponseWriter, err error) {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		WriteJSON(w, appErr.Status, errorBody{Error: appErr})
		return
	}
	slog.Error("unhandled error", "err", err)
	WriteJSON(w, http.StatusInternalServerError, errorBody{
		Error: apperr.New(http.StatusInternalServerError, "INTERNAL", "Something went wrong. Please try again."),
	})
}

// DecodeJSON decodes a size-limited JSON body and rejects unknown fields.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperr.BadRequest("Invalid request body: " + err.Error())
	}
	return nil
}
