package staff

import (
	"context"
	"errors"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/tokens"
)

// Resolve returns who a session token belongs to. It refuses a token that is
// unknown, ended, idle for too long or past its absolute limit, and says
// nothing about which. A session that has not used a passkey still resolves:
// its MFA state says what is allowed.
func (s *Service) Resolve(ctx context.Context, token string) (Principal, error) {
	sess, err := s.d.Repo.FindSession(ctx, tokens.Hash(token))
	if err != nil {
		return Principal{}, invalidSession(err)
	}
	now := s.d.Clock.Now()
	if sess.RevokedAt != nil || !sess.ExpiresAt.After(now) ||
		now.Sub(sess.LastSeenAt) > s.cfg.Idle {
		return Principal{}, identity.ErrInvalidSession
	}
	u, err := s.d.Repo.Find(ctx, sess.UserID)
	if err != nil || u.Status != StatusActive {
		return Principal{}, invalidSession(err)
	}
	keys, err := s.d.Repo.Passkeys(ctx, u.ID)
	if err != nil {
		return Principal{}, err
	}
	if now.Sub(sess.LastSeenAt) >= s.cfg.TouchEvery {
		_ = s.d.Repo.Touch(ctx, sess.ID, now)
		sess.LastSeenAt = now
	}
	state := s.stateOf(sess, len(keys))
	return Principal{Session: sess, User: u, MFA: state, Passkeys: len(keys)}, nil
}

func (s *Service) stateOf(sess Session, passkeys int) MFAState {
	if sess.MFAAt != nil {
		return MFADone
	}
	return stateFor(passkeys)
}

func invalidSession(err error) error {
	missing := errors.Is(err, identity.ErrInvalidToken) || errors.Is(err, identity.ErrNotFound)
	if err == nil || missing {
		return identity.ErrInvalidSession
	}
	return err
}

// Logout ends the session behind token. An unknown token is not an error.
func (s *Service) Logout(ctx context.Context, token string) error {
	p, err := s.Resolve(ctx, token)
	if errors.Is(err, identity.ErrInvalidSession) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.d.Repo.RevokeSession(ctx, p.User.ID, p.Session.ID, s.d.Clock.Now())
}
