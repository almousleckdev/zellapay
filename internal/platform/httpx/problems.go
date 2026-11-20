package httpx

import "net/http"

var (
	problemInternal = Problem{
		Status:  http.StatusInternalServerError,
		Code:    "internal_error",
		Message: "An internal error occurred.",
	}
	problemTooLarge = Problem{
		Status:  http.StatusRequestEntityTooLarge,
		Code:    "request_too_large",
		Message: "The request body is too large.",
	}
)
