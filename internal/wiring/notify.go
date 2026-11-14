package wiring

import (
	"fmt"
	"log/slog"

	"github.com/zellapay/zellapay/internal/notify"
	"github.com/zellapay/zellapay/internal/notify/logsender"
	notifystore "github.com/zellapay/zellapay/internal/notify/pgstore"
	"github.com/zellapay/zellapay/internal/notify/smtpsender"
	"github.com/zellapay/zellapay/internal/platform/crypto"
)

// buildNotify connects the notification service to its senders. SMS is only
// available where a sender exists: outside production that is the log sender;
// in production there is none until a gateway is chosen, so an SMS cannot be
// queued by mistake.
func (c *Container) buildNotify(
	sealer *crypto.Sealer, log *slog.Logger,
) (*notify.Service, *notifystore.Store, error) {
	email, err := smtpsender.New(smtpsender.Config{
		Addr: c.Settings.SMTPAddr, From: c.Settings.MailFrom,
		Username: c.Settings.SMTPUsername, Password: c.Settings.SMTPPassword,
		RequireTLS: c.Settings.SMTPRequireTLS,
	}, c.Clock)
	if err != nil {
		return nil, nil, fmt.Errorf("email: %w", err)
	}
	senders := map[notify.Channel]notify.Sender{notify.Email: email}
	if c.Settings.SMSLogOnly {
		senders[notify.SMS] = logsender.New(log)
	}
	store := notifystore.New(c.Pool)
	svc, err := notify.NewService(notify.Deps{
		Repo: store, Senders: senders, Box: sealer, Clock: c.Clock, Log: log,
	}, notify.DefaultConfig())
	return svc, store, err
}
