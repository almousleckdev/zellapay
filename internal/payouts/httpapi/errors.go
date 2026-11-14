package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/zellapay/zellapay/internal/payouts"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

var (
	errNoMerchant = errors.New("httpapi: no authenticated merchant")

	problemBadJSON = httpx.Problem{
		Status: http.StatusBadRequest, Code: "invalid_request",
		Message: "The request body must be one valid JSON object with known fields.",
	}
	problemBadBatch = httpx.Problem{
		Status: http.StatusBadRequest, Code: "invalid_request",
		Message: "A batch needs between 1 and 100 payouts, each with known fields.",
	}
	problemBadQuery = httpx.Problem{
		Status: http.StatusBadRequest, Code: "invalid_request",
		Message: "A query parameter is invalid.",
	}
	problemNotFound = httpx.Problem{
		Status: http.StatusNotFound, Code: "not_found", Message: "No such payout.",
	}
	problemUnsupported = httpx.Problem{
		Status: http.StatusUnprocessableEntity, Code: "unsupported_number",
		Message: "No operator serves this phone number.",
	}
	problemFunds = httpx.Problem{
		Status: http.StatusUnprocessableEntity, Code: "insufficient_funds",
		Message: "The available balance does not cover the amount and fee.",
	}
	problemTooLarge = httpx.Problem{
		Status: http.StatusUnprocessableEntity, Code: "amount_too_large",
		Message: "The amount exceeds the limit for a single payout.",
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

// problemFor maps a service error to a client-safe problem. Unexpected
// errors are logged with the request ID and never shown to the client.
func (h *Handler) problemFor(r *http.Request, err error) httpx.Problem {
	switch {
	case errors.Is(err, payouts.ErrInvalidInput):
		return invalidInput(err)
	case errors.Is(err, payouts.ErrUnsupportedNumber):
		return problemUnsupported
	case errors.Is(err, payouts.ErrInsufficientFunds):
		return problemFunds
	case errors.Is(err, payouts.ErrAmountTooLarge):
		return problemTooLarge
	case errors.Is(err, payouts.ErrIdempotencyConflict):
		return problemKeyReused
	case errors.Is(err, payouts.ErrNotFound):
		return problemNotFound
	}
	h.log.Error("request failed", "err", err, "request_id", httpx.RequestID(r.Context()))
	return problemInternal
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpx.WriteError(w, r, h.problemFor(r, err))
}

// invalidInput turns a validation error into a 400. The text after the
// sentinel is written by this codebase and is safe to show.
func invalidInput(err error) httpx.Problem {
	detail := strings.TrimPrefix(err.Error(), payouts.ErrInvalidInput.Error())
	detail = strings.TrimPrefix(detail, ": ")
	if detail == "" {
		detail = "The request is invalid."
	}
	return httpx.Problem{Status: http.StatusBadRequest, Code: "invalid_request", Message: detail}
}
