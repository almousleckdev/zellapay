package notify

import "errors"

// Errors returned by the module.
var (
	ErrInvalidInput       = errors.New("notify: invalid input")
	ErrChannelUnavailable = errors.New("notify: no sender is configured for this channel")
	ErrNotFound           = errors.New("notify: not found")
)

// SendError is a failure to send, reduced to a stable code that is safe to
// store. Permanent failures are not retried: the recipient was refused.
type SendError struct {
	Code      string
	Permanent bool
}

func (e *SendError) Error() string { return "notify: send failed: " + e.Code }

// Send failure codes.
const (
	CodeUnavailable = "unavailable" // connection or server trouble; retry
	CodeTimeout     = "timeout"
	CodeRejected    = "rejected" // the recipient or content was refused
	CodeUnreadable  = "payload_unreadable"
	CodeUnknown     = "send_error"
)
