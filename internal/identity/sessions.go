package identity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/zellapay/zellapay/internal/platform/clock"
	"github.com/zellapay/zellapay/internal/platform/ids"
)

// Authenticator checks an email and a password.
type Authenticator interface {
	Authenticate(ctx context.Context, email, password string) (User, error)
}

// SecondFactor enrols and checks the user's second factor.
type SecondFactor interface {
	Enrolled(ctx context.Context, userID string) (bool, error)
	BeginEnrolment(ctx context.Context, userID string) (Enrolment, error)
	ConfirmEnrolment(ctx context.Context, userID, code string) ([]string, error)
	Verify(ctx context.Context, userID, code string) (Verification, error)
}

// SessionDeps are the collaborators of the Sessions service.
type SessionDeps struct {
	Repo     SessionRepository
	Users    UserLookup
	Accounts Authenticator
	Factor   SecondFactor
	Throttle Throttle
	Clock    clock.Clock
	Log      *slog.Logger
}

// Sessions signs people in and keeps them signed in.
type Sessions struct {
	d   SessionDeps
	cfg SessionConfig
}

// NewSessions returns a Sessions service.
func NewSessions(d SessionDeps, cfg SessionConfig) (*Sessions, error) {
	if !d.complete() {
		return nil, errors.New("identity: missing session dependency")
	}
	if !cfg.valid() {
		return nil, errors.New("identity: invalid session configuration")
	}
	return &Sessions{d: d, cfg: cfg}, nil
}

func (d SessionDeps) complete() bool {
	return d.Repo != nil && d.Users != nil && d.Accounts != nil && d.Factor != nil &&
		d.Throttle != nil && d.Clock != nil && d.Log != nil
}

func (c SessionConfig) valid() bool {
	return c.Idle > 0 && c.Absolute >= c.Idle && c.TouchEvery >= 0
}

// LoginInput is a sign-in attempt.
type LoginInput struct {
	Email    string
	Password string
	Source   Source
}

// Issued is a new session. Token is shown to the caller once and never
// stored; the frontend keeps it in an encrypted, HttpOnly cookie.
type Issued struct {
	Token   string
	Session Session
	MFA     MFAState
}

// Login checks the password and opens a session. The session starts without
// the second factor: MFA says what it still needs. Repeated failures for an
// account or from an address are slowed down before the password is looked at.
func (s *Sessions) Login(ctx context.Context, in LoginInput) (Issued, error) {
	a := Attempt{
		Realm:   RealmPassword,
		Account: strings.ToLower(strings.TrimSpace(in.Email)), IP: in.Source.IP,
	}
	if err := s.guard(ctx, a); err != nil {
		return Issued{}, err
	}
	u, err := s.d.Accounts.Authenticate(ctx, in.Email, in.Password)
	if err != nil {
		return Issued{}, s.failed(ctx, a, err)
	}
	s.succeeded(ctx, a)
	state, err := s.stateOf(ctx, u.ID)
	if err != nil {
		return Issued{}, err
	}
	return s.open(ctx, u, in.Source, state)
}

func (s *Sessions) stateOf(ctx context.Context, userID string) (MFAState, error) {
	on, err := s.d.Factor.Enrolled(ctx, userID)
	if err != nil {
		return "", err
	}
	if on {
		return MFARequired, nil
	}
	return MFAEnrolRequired, nil
}

func (s *Sessions) open(
	ctx context.Context, u User, src Source, state MFAState,
) (Issued, error) {
	token, hash, err := newToken()
	if err != nil {
		return Issued{}, err
	}
	now := s.d.Clock.Now()
	sess := Session{
		ID: ids.New("ses_"), UserID: u.ID, TokenHash: hash, CreatedAt: now,
		LastSeenAt: now, ExpiresAt: now.Add(s.cfg.Absolute), Source: src,
	}
	if err := s.d.Repo.CreateSession(ctx, sess); err != nil {
		return Issued{}, fmt.Errorf("create session: %w", err)
	}
	return Issued{Token: token, Session: sess, MFA: state}, nil
}
