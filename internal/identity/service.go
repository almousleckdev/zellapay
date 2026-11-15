package identity

import (
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/zellapay/zellapay/internal/platform/clock"
)

// Deps are the collaborators a Service needs.
type Deps struct {
	Repo      Repository
	Hasher    *Hasher
	Mailer    Mailer
	Auditor   Auditor
	Directory Directory
	Clock     clock.Clock
	Log       *slog.Logger
}

// Config holds the settings of the links sent to users.
type Config struct {
	// DashboardURL is the merchant dashboard's public address, without a
	// trailing slash. Links in emails always start with it.
	DashboardURL    string
	InvitationTTL   time.Duration
	VerificationTTL time.Duration
	ResetTTL        time.Duration
}

// DefaultConfig returns the lifetimes from the design.
func DefaultConfig(dashboardURL string) Config {
	return Config{
		DashboardURL:    strings.TrimRight(dashboardURL, "/"),
		InvitationTTL:   7 * 24 * time.Hour,
		VerificationTTL: 24 * time.Hour,
		ResetTTL:        30 * time.Minute,
	}
}

// Service is the identity use cases.
type Service struct {
	d   Deps
	cfg Config
}

// NewService returns a Service.
func NewService(d Deps, cfg Config) (*Service, error) {
	if !d.complete() {
		return nil, errors.New("identity: missing dependency")
	}
	if !cfg.valid() {
		return nil, errors.New("identity: invalid configuration")
	}
	return &Service{d: d, cfg: cfg}, nil
}

func (d Deps) complete() bool {
	return d.Repo != nil && d.Hasher != nil && d.Mailer != nil && d.Auditor != nil &&
		d.Directory != nil && d.Clock != nil && d.Log != nil
}

func (c Config) valid() bool {
	return strings.HasPrefix(c.DashboardURL, "http") &&
		c.InvitationTTL > 0 && c.VerificationTTL > 0 && c.ResetTTL > 0
}

func (s *Service) now() time.Time { return s.d.Clock.Now() }

// link builds an emailed link. The token is URL-safe, so it needs no escaping.
func (s *Service) link(path, token string) string {
	return s.cfg.DashboardURL + path + "?token=" + token
}
