// Package logsender "sends" messages by writing them to the log. It is for
// development and tests, so that codes and links can be read without a
// gateway. Nothing is delivered.
package logsender

import (
	"context"
	"log/slog"

	"github.com/zellapay/zellapay/internal/notify"
	"github.com/zellapay/zellapay/internal/platform/phone"
)

// Sender implements notify.Sender by logging.
type Sender struct {
	log *slog.Logger
}

var _ notify.Sender = (*Sender)(nil)

// New returns a Sender that writes to log. Only construct it in development:
// it writes message bodies, which may contain codes and links.
func New(log *slog.Logger) *Sender { return &Sender{log: log} }

// Send writes the message to the log. The recipient number is masked; the body
// is shown in full so a developer can use the code or link in it.
func (s *Sender) Send(_ context.Context, d notify.Delivery) error {
	s.log.Info("sms (development only, not sent)",
		"id", d.ID, "to", phone.Mask(d.To), "body", d.Body)
	return nil
}
