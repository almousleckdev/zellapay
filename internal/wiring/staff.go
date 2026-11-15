package wiring

import (
	"fmt"
	"log/slog"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/staff"
	staffstore "github.com/zellapay/zellapay/internal/staff/pgstore"
)

// buildStaff connects the staff module. It shares the password hasher and the
// throttle with merchant sign-in, so one policy covers both.
func (c *Container) buildStaff(
	hasher *identity.Hasher, limiter identity.Throttle, log *slog.Logger,
) error {
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "ZellaPay Admin",
		RPID:          c.Settings.WebAuthnRPID,
		RPOrigins:     []string{c.Settings.AdminURL},
	})
	if err != nil {
		return fmt.Errorf("webauthn: %w", err)
	}
	c.Staff, err = staff.NewService(staff.Deps{
		Repo: staffstore.New(c.Pool, c.AuditStore), Hasher: hasher, Auditor: c.Audit,
		Throttle: limiter, WebAuthn: wa, Clock: c.Clock, Log: log,
	}, staff.DefaultConfig())
	return err
}
