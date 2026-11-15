package keysapi

import (
	"errors"
	"net/http"

	"github.com/zellapay/zellapay/internal/access/accesshttp"
	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

var (
	problemBadJSON = httpx.Problem{
		Status: http.StatusBadRequest, Code: "invalid_request",
		Message: "The request body must be one valid JSON object with known fields.",
	}
	problemInvalid = httpx.Problem{
		Status: http.StatusBadRequest, Code: "invalid_request",
		Message: "The label is too long, or a scope or duration is not allowed.",
	}
	problemNotFound = httpx.Problem{
		Status: http.StatusNotFound, Code: "not_found", Message: "No such API key.",
	}
	problemInternal = httpx.Problem{
		Status: http.StatusInternalServerError, Code: "internal_error",
		Message: "An internal error occurred.",
	}
)

// fail writes the response for an error. Unexpected errors are logged with
// the request ID and never shown.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	if p, ok := accesshttp.Problem(err); ok {
		httpx.WriteError(w, r, p)
		return
	}
	switch {
	case errors.Is(err, merchants.ErrInvalidInput):
		httpx.WriteError(w, r, problemInvalid)
	case errors.Is(err, merchants.ErrNotFound):
		httpx.WriteError(w, r, problemNotFound)
	default:
		h.d.Log.Error("request failed", "err", err, "request_id", httpx.RequestID(r.Context()))
		httpx.WriteError(w, r, problemInternal)
	}
}
