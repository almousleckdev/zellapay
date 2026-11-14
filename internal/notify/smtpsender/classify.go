package smtpsender

import (
	"context"
	"errors"
	"net"
	"net/textproto"

	"github.com/zellapay/zellapay/internal/notify"
)

// classify reduces an SMTP or network error to a stable code. Raw errors can
// contain server banners and addresses and are never stored.
//
// A 5xx reply to a recipient or to the message is permanent: the address or
// content was refused, so retrying only wastes the sender's reputation.
// Authentication and policy replies point at our own configuration, so they
// are retried (and noticed) rather than discarding the person's message.
func classify(err error) *notify.SendError {
	var proto *textproto.Error
	var netErr net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()):
		return &notify.SendError{Code: notify.CodeTimeout}
	case errors.As(err, &proto):
		return fromReply(proto.Code)
	}
	return &notify.SendError{Code: notify.CodeUnavailable}
}

func fromReply(code int) *notify.SendError {
	switch {
	case code == 530 || code == 534 || code == 535 || code == 538:
		return &notify.SendError{Code: notify.CodeUnavailable}
	case code >= 500:
		return &notify.SendError{Code: notify.CodeRejected, Permanent: true}
	}
	return &notify.SendError{Code: notify.CodeUnavailable}
}
