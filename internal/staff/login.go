package staff

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/ids"
	"github.com/zellapay/zellapay/internal/platform/tokens"
)

// LoginInput is a sign-in attempt.
type LoginInput struct {
	Email    string
	Password string
	Source   identity.Source
}

// Issued is a new session. Token is shown once and never stored.
type Issued struct {
	Token   string
	Session Session
	MFA     MFAState
}

// attempt names a throttled try. Staff counters never mix with merchants'.
func attempt(realm identity.Realm, account, ip string) identity.Attempt {
	return identity.Attempt{Realm: realm, Account: "staff:" + strings.ToLower(account), IP: ip}
}

// Login checks the password and opens a session that has not used a passkey.
// Repeated failures are slowed down before the password is looked at, and
// every failure looks the same.
func (s *Service) Login(ctx context.Context, in LoginInput) (Issued, error) {
	a := attempt(identity.RealmPassword, strings.TrimSpace(in.Email), in.Source.IP)
	if err := s.guard(ctx, a); err != nil {
		return Issued{}, err
	}
	u, err := s.authenticate(ctx, in.Email, in.Password)
	if err != nil {
		return Issued{}, s.failed(ctx, a, err)
	}
	s.succeeded(ctx, a)
	keys, err := s.d.Repo.Passkeys(ctx, u.ID)
	if err != nil {
		return Issued{}, err
	}
	return s.open(ctx, u, in.Source, stateFor(len(keys)))
}

func stateFor(passkeys int) MFAState {
	if passkeys == 0 {
		return MFAEnrolRequired
	}
	return MFARequired
}

func (s *Service) authenticate(ctx context.Context, email, password string) (User, error) {
	addr, err := identity.NormalizeEmail(email)
	if err != nil {
		s.d.Hasher.SpendTime(password)
		return User{}, identity.ErrInvalidCredentials
	}
	u, err := s.d.Repo.FindByEmail(ctx, addr)
	if errors.Is(err, identity.ErrNotFound) {
		s.d.Hasher.SpendTime(password)
		return User{}, identity.ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}
	ok, stale, err := s.d.Hasher.Verify(password, u.PasswordHash)
	if err != nil || !ok || u.Status != StatusActive {
		return User{}, identity.ErrInvalidCredentials
	}
	if stale {
		s.rehash(ctx, u, password)
	}
	return u, nil
}

func (s *Service) rehash(ctx context.Context, u User, password string) {
	fresh, err := s.d.Hasher.Hash(password)
	if err == nil {
		err = s.d.Repo.Rehash(ctx, u.ID, u.PasswordHash, fresh)
	}
	if err != nil {
		s.d.Log.Warn("staff password rehash failed", "user_id", u.ID, "error", err)
	}
}

func (s *Service) open(
	ctx context.Context, u User, src identity.Source, state MFAState,
) (Issued, error) {
	token, hash, err := tokens.New()
	if err != nil {
		return Issued{}, err
	}
	now := s.d.Clock.Now()
	sess := Session{
		ID: ids.New("ssn_"), UserID: u.ID, TokenHash: hash, CreatedAt: now, LastSeenAt: now,
		ExpiresAt: now.Add(s.cfg.Absolute), IP: src.IP, UserAgent: src.UserAgent,
	}
	if err := s.d.Repo.CreateSession(ctx, sess); err != nil {
		return Issued{}, fmt.Errorf("create session: %w", err)
	}
	return Issued{Token: token, Session: sess, MFA: state}, nil
}

// guard refuses an attempt that is being throttled, and fails closed.
func (s *Service) guard(ctx context.Context, a identity.Attempt) error {
	wait, err := s.d.Throttle.Wait(ctx, a)
	if err != nil {
		return err
	}
	if wait > 0 {
		return &identity.ThrottledError{RetryAfter: wait}
	}
	return nil
}

// failed counts a wrong answer and returns the error to show.
func (s *Service) failed(ctx context.Context, a identity.Attempt, cause error) error {
	wrong := errors.Is(cause, identity.ErrInvalidCredentials) ||
		errors.Is(cause, identity.ErrInvalidCode)
	if !wrong {
		return cause
	}
	if err := s.d.Throttle.Failed(ctx, a); err != nil {
		s.d.Log.Error("throttle: record failure", "realm", a.Realm, "error", err)
	}
	return cause
}

func (s *Service) succeeded(ctx context.Context, a identity.Attempt) {
	if err := s.d.Throttle.Succeeded(ctx, a); err != nil {
		s.d.Log.Warn("throttle: clear failures", "realm", a.Realm, "error", err)
	}
}
