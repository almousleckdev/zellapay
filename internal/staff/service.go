package staff

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/clock"
	"github.com/zellapay/zellapay/internal/platform/ids"
)

// Auditor prepares audit entries.
type Auditor interface {
	Prepare(e audit.Entry) (audit.Pending, error)
}

// Deps are the collaborators of Service.
type Deps struct {
	Repo     Repository
	Hasher   *identity.Hasher
	Auditor  Auditor
	Throttle identity.Throttle
	WebAuthn *webauthn.WebAuthn
	Clock    clock.Clock
	Log      *slog.Logger
}

// Service is the staff use cases.
type Service struct {
	d   Deps
	cfg Config
}

// NewService returns a Service.
func NewService(d Deps, cfg Config) (*Service, error) {
	if !d.complete() {
		return nil, errors.New("staff: missing dependency")
	}
	if cfg.Idle <= 0 || cfg.Absolute < cfg.Idle || cfg.Challenge <= 0 {
		return nil, errors.New("staff: invalid configuration")
	}
	return &Service{d: d, cfg: cfg}, nil
}

func (d Deps) complete() bool {
	return d.Repo != nil && d.Hasher != nil && d.Auditor != nil && d.Throttle != nil &&
		d.WebAuthn != nil && d.Clock != nil && d.Log != nil
}

// CreateInput describes a new staff account and who is creating it.
type CreateInput struct {
	Email    string
	Name     string
	Role     Role
	Password string
	By       audit.Actor
	// Reason is recorded in the audit log. Staff actors must give one.
	Reason string
}

// Create adds a staff account. The first account is made from the command
// line with the system as the actor; later ones by an existing administrator.
func (s *Service) Create(ctx context.Context, in CreateInput) (User, error) {
	email, err := identity.NormalizeEmail(in.Email)
	name := strings.TrimSpace(in.Name)
	if err != nil || name == "" || len(name) > 100 || !in.Role.Valid() {
		return User{}, fmt.Errorf("%w: email, name or role", identity.ErrInvalidInput)
	}
	if err := identity.CheckPassword(in.Password, email, name); err != nil {
		return User{}, err
	}
	hash, err := s.d.Hasher.Hash(in.Password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}
	now := s.d.Clock.Now()
	u := User{
		ID: ids.New("stf_"), Email: email, Name: name, Role: in.Role, Status: StatusActive,
		PasswordHash: hash, PasswordChangedAt: now, CreatedBy: in.By.ID, CreatedAt: now,
	}
	entry, err := s.d.Auditor.Prepare(audit.Entry{
		Actor: in.By, Action: "staff.created", Reason: in.Reason,
		Target: audit.Target{Kind: "staff_user", ID: u.ID},
		Change: map[string]string{"role": string(u.Role)},
	})
	if err != nil {
		return User{}, fmt.Errorf("prepare audit entry: %w", err)
	}
	if err := s.d.Repo.CreateUser(ctx, u, Effects{Audit: []audit.Pending{entry}}); err != nil {
		return User{}, err
	}
	return u, nil
}
