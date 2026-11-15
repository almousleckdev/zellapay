package identity

import (
	"context"
	"errors"
)

// Resolve returns who a session token belongs to. It refuses a token that is
// unknown, ended, idle for too long or past its absolute limit, and says
// nothing about which. A session that has not finished MFA still resolves:
// its MFA state tells the caller what is allowed.
func (s *Sessions) Resolve(ctx context.Context, token string) (Principal, error) {
	sess, err := s.d.Repo.FindSession(ctx, hashToken(token))
	if err != nil {
		return Principal{}, s.invalid(err)
	}
	now := s.d.Clock.Now()
	if sess.RevokedAt != nil || !sess.ExpiresAt.After(now) ||
		now.Sub(sess.LastSeenAt) > s.cfg.Idle {
		return Principal{}, ErrInvalidSession
	}
	u, err := s.d.Users.FindUser(ctx, sess.UserID)
	if err != nil || u.Status != StatusActive {
		return Principal{}, s.invalid(err)
	}
	state, err := s.principalState(ctx, sess)
	if err != nil {
		return Principal{}, err
	}
	if now.Sub(sess.LastSeenAt) >= s.cfg.TouchEvery {
		_ = s.d.Repo.Touch(ctx, sess.ID, now)
		sess.LastSeenAt = now
	}
	return Principal{Session: sess, User: u, MFA: state}, nil
}

func (s *Sessions) principalState(ctx context.Context, sess Session) (MFAState, error) {
	if sess.MFAAt != nil {
		return MFADone, nil
	}
	return s.stateOf(ctx, sess.UserID)
}

// invalid maps "not found" and similar to the one session error, and lets
// real failures through.
func (s *Sessions) invalid(err error) error {
	if err == nil || errors.Is(err, ErrInvalidToken) || errors.Is(err, ErrNotFound) {
		return ErrInvalidSession
	}
	return err
}

// Logout ends the session behind token. An unknown token is not an error.
func (s *Sessions) Logout(ctx context.Context, token string) error {
	p, err := s.Resolve(ctx, token)
	if errors.Is(err, ErrInvalidSession) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = s.d.Repo.RevokeSession(ctx, p.User.ID, p.Session.ID, s.d.Clock.Now())
	return err
}

// List returns the user's open sessions, most recently used first.
func (s *Sessions) List(ctx context.Context, userID string) ([]Session, error) {
	return s.d.Repo.ListSessions(ctx, userID, s.d.Clock.Now())
}

// End ends one of the user's sessions. It never touches another user's.
func (s *Sessions) End(ctx context.Context, userID, sessionID string) error {
	ok, err := s.d.Repo.RevokeSession(ctx, userID, sessionID, s.d.Clock.Now())
	if err == nil && !ok {
		return ErrNotFound
	}
	return err
}

// EndOthers ends every open session of the user except keepID, and returns
// how many it ended.
func (s *Sessions) EndOthers(ctx context.Context, userID, keepID string) (int, error) {
	return s.d.Repo.RevokeUserSessions(ctx, userID, keepID, s.d.Clock.Now())
}
