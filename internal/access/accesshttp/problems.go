// Package accesshttp turns access-policy refusals into HTTP responses, so
// every route group answers them the same way.
package accesshttp

import (
	"errors"
	"net/http"

	"github.com/zellapay/zellapay/internal/access"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

var (
	mfaRequired = httpx.Problem{
		Status: http.StatusForbidden, Code: "mfa_required",
		Message: "Complete the second factor first.",
	}
	stepUp = httpx.Problem{
		Status: http.StatusForbidden, Code: "step_up_required",
		Message: "Confirm with a recent code to continue.",
	}
	forbidden = httpx.Problem{
		Status: http.StatusForbidden, Code: "forbidden",
		Message: "You are not allowed to do this.",
	}
)

// Problem returns the response for a policy refusal. It reports false for
// any other error.
func Problem(err error) (httpx.Problem, bool) {
	switch {
	case errors.Is(err, access.ErrMFARequired):
		return mfaRequired, true
	case errors.Is(err, access.ErrStepUpRequired):
		return stepUp, true
	case errors.Is(err, access.ErrForbidden):
		return forbidden, true
	}
	return httpx.Problem{}, false
}
