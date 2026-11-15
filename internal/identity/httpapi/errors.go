package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/zellapay/zellapay/internal/access/accesshttp"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

func problem(status int, code, message string) httpx.Problem {
	return httpx.Problem{Status: status, Code: code, Message: message}
}

var (
	problemBadJSON = problem(http.StatusBadRequest, "invalid_request",
		"The request body must be one valid JSON object with known fields.")
	problemBadToken = problem(http.StatusBadRequest, "invalid_link",
		"This link is invalid or has expired.")
	problemCredentials = problem(http.StatusUnauthorized, "invalid_credentials",
		"The email or password is incorrect.")
	problemSession = problem(http.StatusUnauthorized, "invalid_session",
		"The session is missing, expired or ended.")
	problemCode = problem(http.StatusUnprocessableEntity, "invalid_code",
		"The code is incorrect or was already used.")
	problemExists = problem(http.StatusConflict, "account_exists",
		"An account already exists for this email.")
	problemNotFound  = problem(http.StatusNotFound, "not_found", "No such session.")
	problemForbidden = problem(http.StatusForbidden, "forbidden",
		"You are not allowed to do this.")
	problemInternal = problem(http.StatusInternalServerError, "internal_error",
		"An internal error occurred.")
)

// fail writes the response for an error. Unexpected errors are logged with
// the request ID and never shown to the client.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	var throttled *identity.ThrottledError
	if errors.As(err, &throttled) {
		secs := int(throttled.RetryAfter.Seconds()) + 1
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		httpx.WriteError(w, r, problem(http.StatusTooManyRequests, "too_many_attempts",
			"Too many attempts. Try again later."))
		return
	}
	if p, ok := accesshttp.Problem(err); ok {
		httpx.WriteError(w, r, p)
		return
	}
	if p, ok := mapped(err); ok {
		httpx.WriteError(w, r, p)
		return
	}
	h.d.Log.Error("request failed", "err", err, "request_id", httpx.RequestID(r.Context()))
	httpx.WriteError(w, r, problemInternal)
}

func mapped(err error) (httpx.Problem, bool) {
	for _, m := range knownErrors {
		if errors.Is(err, m.err) {
			return m.p, true
		}
	}
	if errors.Is(err, identity.ErrWeakPassword) {
		return problem(http.StatusUnprocessableEntity, "weak_password", safeDetail(err)), true
	}
	if errors.Is(err, identity.ErrInvalidInput) {
		return problem(http.StatusBadRequest, "invalid_request", safeDetail(err)), true
	}
	return httpx.Problem{}, false
}

var knownErrors = []struct {
	err error
	p   httpx.Problem
}{
	{identity.ErrInvalidToken, problemBadToken},
	{identity.ErrInvalidCredentials, problemCredentials},
	{identity.ErrInvalidSession, problemSession},
	{identity.ErrInvalidCode, problemCode},
	{identity.ErrEmailTaken, problemExists},
	{identity.ErrNotFound, problemNotFound},
	{identity.ErrMFAAlreadyEnrolled, problemForbidden},
	{identity.ErrNoEnrolment, problemForbidden},
}

// safeDetail returns the part of a validation error this codebase wrote,
// which is meant to be shown.
func safeDetail(err error) string {
	_, detail, ok := strings.Cut(err.Error(), ": ")
	if !ok {
		return "The request is invalid."
	}
	if _, rest, more := strings.Cut(detail, ": "); more {
		return rest
	}
	return detail
}
