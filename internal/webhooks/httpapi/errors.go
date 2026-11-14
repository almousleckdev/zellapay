package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/zellapay/zellapay/internal/platform/httpx"
	"github.com/zellapay/zellapay/internal/webhooks"
)

var (
	errNoMerchant = errors.New("httpapi: no authenticated merchant")

	problemBadJSON = httpx.Problem{
		Status: http.StatusBadRequest, Code: "invalid_request",
		Message: "The request body must be one valid JSON object with known fields.",
	}
	problemNotFound = httpx.Problem{
		Status: http.StatusNotFound, Code: "not_found", Message: "No such event.",
	}
	problemUnsafeURL = httpx.Problem{
		Status: http.StatusUnprocessableEntity, Code: "unsafe_url",
		Message: "The URL must be a public https address.",
	}
	problemLimit = httpx.Problem{
		Status: http.StatusUnprocessableEntity, Code: "endpoint_limit_reached",
		Message: "You have reached the maximum number of webhook endpoints.",
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
	case errors.Is(err, webhooks.ErrInvalidInput):
		httpx.WriteError(w, r, invalidInput(err))
	case errors.Is(err, webhooks.ErrUnsafeURL):
		httpx.WriteError(w, r, problemUnsafeURL)
	case errors.Is(err, webhooks.ErrTooManyEndpoints):
		httpx.WriteError(w, r, problemLimit)
	case errors.Is(err, webhooks.ErrNotFound):
		httpx.WriteError(w, r, problemNotFound)
	default:
		h.log.Error("request failed", "err", err, "request_id", httpx.RequestID(r.Context()))
		httpx.WriteError(w, r, problemInternal)
	}
}

// invalidInput turns a validation error into a 400. The text after the
// sentinel is written by this codebase and is safe to show.
func invalidInput(err error) httpx.Problem {
	detail := strings.TrimPrefix(err.Error(), webhooks.ErrInvalidInput.Error())
	detail = strings.TrimPrefix(detail, ": ")
	if detail == "" {
		detail = "The request is invalid."
	}
	return httpx.Problem{Status: http.StatusBadRequest, Code: "invalid_request", Message: detail}
}
