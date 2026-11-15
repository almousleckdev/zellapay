package identity

import (
	"context"
	"errors"
)

// Authenticate checks an email and password. Every failure returns the same
// error after the same amount of work, so the answer never reveals whether
// the address has an account. Throttling and sessions are applied by callers.
func (s *Service) Authenticate(ctx context.Context, email, password string) (User, error) {
	addr, err := NormalizeEmail(email)
	if err != nil {
		s.d.Hasher.SpendTime(password)
		return User{}, ErrInvalidCredentials
	}
	u, err := s.d.Repo.FindUserByEmail(ctx, addr)
	if errors.Is(err, ErrNotFound) {
		s.d.Hasher.SpendTime(password)
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}
	ok, stale, err := s.d.Hasher.Verify(password, u.PasswordHash)
	if err != nil || !ok || u.Status != StatusActive {
		return User{}, ErrInvalidCredentials
	}
	if stale {
		s.rehash(ctx, u, password)
	}
	return u, nil
}

// rehash upgrades a hash made with older settings. A failure is logged and
// does not block the sign-in: the old hash still works.
func (s *Service) rehash(ctx context.Context, u User, password string) {
	fresh, err := s.d.Hasher.Hash(password)
	if err == nil {
		err = s.d.Repo.Rehash(ctx, u.ID, u.PasswordHash, fresh)
	}
	if err != nil {
		s.d.Log.Warn("password rehash failed", "user_id", u.ID, "error", err)
	}
}
