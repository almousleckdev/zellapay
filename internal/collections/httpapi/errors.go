package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

var (
	errNoMerchant = errors.New("httpapi: no authenticated merchant")
	errBadStatus  = errors.New("httpapi: unknown status filter")

	problemBadJSON = httpx.Problem{
		Status: http.StatusBadRequest, Code: "invalid_request",
		Message: "The request body must be one valid JSON object with known fields.",
	}
	problemBadQuery = httpx.Problem{
		Status: http.StatusBadRequest, Code: "invalid_request",
		Message: "A query parameter is invalid.",
	}
	problemNotFound = httpx.Problem{
		Status: http.StatusNotFound, Code: "not_found",
		Message: "No such collection.",
	}
	problemUnsupported = httpx.Problem{
		Status: http.StatusUnprocessableEntity, Code: "unsupported_number",
		Message: "No operator serves this phone number.",
	}
	problemKeyReused = httpx.Problem{
		Status: http.StatusUnprocessableEntity, Code: "idempotency_key_reused",
		Message: "This Idempotency-Key was already used with a different request.",
	}
	problemInternal = httpx.Problem{
		Status: http.StatusInternalServerError, Code: "internal_error",
		Message: "An internal error occurred.",
	}
)

// fail writes the response for a service error. Unexpected errors are
// logged with the request ID and never shown to the client.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, collections.ErrInvalidInput):
		httpx.WriteError(w, r, invalidInput(err))
	case errors.Is(err, collections.ErrUnsupportedNumber):
		httpx.WriteError(w, r, problemUnsupported)
	case errors.Is(err, collections.ErrIdempotencyConflict):
		httpx.WriteError(w, r, problemKeyReused)
	case errors.Is(err, collections.ErrNotFound):
		httpx.WriteError(w, r, problemNotFound)
	default:
		h.log.Error("request failed", "err", err, "request_id", httpx.RequestID(r.Context()))
		httpx.WriteError(w, r, problemInternal)
	}
}

// invalidInput turns a validation error into a 400. The text after the
// sentinel is written by this codebase and is safe to show.
func invalidInput(err error) httpx.Problem {
	detail := strings.TrimPrefix(err.Error(), collections.ErrInvalidInput.Error())
	detail = strings.TrimPrefix(detail, ": ")
	if detail == "" {
		detail = "The request is invalid."
	}
	return httpx.Problem{
		Status: http.StatusBadRequest, Code: "invalid_request", Message: detail,
	}
}
