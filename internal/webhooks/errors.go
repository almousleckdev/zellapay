package webhooks

import "errors"

// Errors returned by the module.
var (
	ErrInvalidInput     = errors.New("webhooks: invalid input")
	ErrNotFound         = errors.New("webhooks: not found")
	ErrTooManyEndpoints = errors.New("webhooks: endpoint limit reached")
	ErrUnsafeURL        = errors.New("webhooks: url is not allowed")
	ErrInvalidSignature = errors.New("webhooks: invalid signature")
	ErrSignatureTooOld  = errors.New("webhooks: signature timestamp outside tolerance")
	ErrMalformedHeader  = errors.New("webhooks: malformed signature header")
)

// SendError is a failure to deliver, reduced to a stable code that is safe
// to store and show: raw network errors can contain internal addresses.
type SendError struct{ Code string }

func (e *SendError) Error() string { return "webhooks: send failed: " + e.Code }

// Send failure codes.
const (
	CodeTimeout     = "timeout"
	CodeConnection  = "connection_failed"
	CodeBlockedHost = "blocked_address"
	CodeTLS         = "tls_error"
	CodeRedirect    = "redirect_not_followed"
	CodeUnknown     = "send_error"
)
