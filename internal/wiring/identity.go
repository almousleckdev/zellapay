package wiring

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/zellapay/zellapay/internal/identity"
	identitystore "github.com/zellapay/zellapay/internal/identity/pgstore"
	"github.com/zellapay/zellapay/internal/identity/redisthrottle"
	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/platform/crypto"
)

// merchantDirectory answers identity's one question about merchants.
type merchantDirectory struct{ svc *merchants.Service }

var _ identity.Directory = merchantDirectory{}

func (d merchantDirectory) BusinessName(ctx context.Context, id string) (string, error) {
	m, err := d.svc.GetMerchant(ctx, id)
	if errors.Is(err, merchants.ErrNotFound) {
		return "", identity.ErrNotFound
	}
	return m.Name, err
}

// secrets are the keys identity needs to protect what it stores.
type secrets struct {
	sealer  *crypto.Sealer
	indexer *crypto.BlindIndexer
}

func (c *Container) buildIdentity(sec secrets, log *slog.Logger) error {
	hasher, err := c.passwordHasher()
	if err != nil {
		return err
	}
	repo := identitystore.New(c.Pool, c.NotifyStore, c.AuditStore)
	c.Identity, err = identity.NewService(identity.Deps{
		Repo: repo, Hasher: hasher, Mailer: c.Notify, Auditor: c.Audit,
		Directory: merchantDirectory{svc: c.Merchants}, Clock: c.Clock, Log: log,
	}, identity.DefaultConfig(c.Settings.DashboardURL))
	if err != nil {
		return err
	}
	c.MFA, err = identity.NewMFA(identity.MFADeps{
		Repo: repo, Users: repo, Box: sec.sealer, Auditor: c.Audit, Clock: c.Clock,
		Issuer: "ZellaPay",
	})
	if err != nil {
		return err
	}
	limiter := redisthrottle.New(c.Redis, sec.indexer, c.Clock, redisthrottle.DefaultPolicy())
	if err := c.buildSessions(repo, limiter, log); err != nil {
		return err
	}
	return c.buildStaff(hasher, limiter, log)
}

func (c *Container) buildSessions(
	repo *identitystore.Store, limiter identity.Throttle, log *slog.Logger,
) (err error) {
	c.Sessions, err = identity.NewSessions(identity.SessionDeps{
		Repo: repo, Users: repo, Accounts: c.Identity, Factor: c.MFA,
		Throttle: limiter, Clock: c.Clock, Log: log,
	}, identity.MerchantSessionConfig())
	return err
}

func (c *Container) passwordHasher() (*identity.Hasher, error) {
	params := c.Settings.PasswordParams
	if params == (identity.Params{}) {
		params = identity.DefaultParams()
	}
	params.Version = c.Settings.PasswordPepperVersion
	h, err := identity.NewHasher(params,
		map[int][]byte{params.Version: c.Settings.PasswordPepper})
	if err != nil {
		return nil, fmt.Errorf("password hasher: %w", err)
	}
	return h, nil
}
