package httpauth

import (
	"errors"
	"net/http"

	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

var (
	problemInvalidKey = httpx.Problem{
		Status:  http.StatusUnauthorized,
		Code:    "invalid_api_key",
		Message: "The API key is missing or invalid.",
	}
	problemInactive = httpx.Problem{
		Status:  http.StatusForbidden,
		Code:    "merchant_inactive",
		Message: "The merchant account is not active.",
	}
	problemIP = httpx.Problem{
		Status:  http.StatusForbidden,
		Code:    "ip_not_allowed",
		Message: "Requests from this address are not permitted for this key.",
	}
	problemScope = httpx.Problem{
		Status:  http.StatusForbidden,
		Code:    "insufficient_scope",
		Message: "The API key does not have the required scope.",
	}
	problemInternal = httpx.Problem{
		Status:  http.StatusInternalServerError,
		Code:    "internal_error",
		Message: "An internal error occurred.",
	}
)

// deny maps an authentication failure to a client-safe response. Wrong,
// unknown, revoked, expired and wrong-environment keys are
// indistinguishable to the caller.
func deny(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, merchants.ErrMerchantInactive):
		httpx.WriteError(w, r, problemInactive)
	case errors.Is(err, merchants.ErrIPNotAllowed):
		httpx.WriteError(w, r, problemIP)
	case errors.Is(err, merchants.ErrInvalidKey),
		errors.Is(err, merchants.ErrWrongMode),
		errors.Is(err, errMissing):
		w.Header().Set("WWW-Authenticate", `Bearer realm="zellapay"`)
		httpx.WriteError(w, r, problemInvalidKey)
	default:
		httpx.WriteError(w, r, problemInternal)
	}
}
