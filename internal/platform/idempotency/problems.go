package idempotency

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/platform/httpx"
)

var (
	problemKeyRequired = httpx.Problem{
		Status:  http.StatusBadRequest,
		Code:    "idempotency_key_required",
		Message: "The Idempotency-Key header is required.",
	}
	problemKeyInvalid = httpx.Problem{
		Status:  http.StatusBadRequest,
		Code:    "idempotency_key_invalid",
		Message: "The Idempotency-Key must be 1 to 255 printable ASCII characters.",
	}
	problemKeyInUse = httpx.Problem{
		Status:  http.StatusConflict,
		Code:    "idempotency_key_in_use",
		Message: "A request with this Idempotency-Key is still being processed.",
	}
	problemKeyReused = httpx.Problem{
		Status:  http.StatusUnprocessableEntity,
		Code:    "idempotency_key_reused",
		Message: "This Idempotency-Key was already used with a different request.",
	}
	problemTooLarge = httpx.Problem{
		Status:  http.StatusRequestEntityTooLarge,
		Code:    "request_too_large",
		Message: "The request body is too large.",
	}
	problemInternal = httpx.Problem{
		Status:  http.StatusInternalServerError,
		Code:    "internal_error",
		Message: "An internal error occurred.",
	}
)
