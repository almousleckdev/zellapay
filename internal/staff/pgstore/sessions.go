package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/staff"
	"github.com/zellapay/zellapay/internal/staff/pgstore/sqlcgen"
)

// CreateSession stores a new session.
func (s *Store) CreateSession(ctx context.Context, sess staff.Session) error {
	return sqlcgen.New(s.pool).InsertStaffSession(ctx, sqlcgen.InsertStaffSessionParams{
		ID: sess.ID, UserID: sess.UserID, TokenHash: sess.TokenHash[:],
		CreatedAt: ts(sess.CreatedAt), ExpiresAt: ts(sess.ExpiresAt),
		Ip: sess.IP, UserAgent: sess.UserAgent,
	})
}

// FindSession returns the session for a token hash.
func (s *Store) FindSession(ctx context.Context, hash [32]byte) (staff.Session, error) {
	r, err := sqlcgen.New(s.pool).GetStaffSessionByHash(ctx, hash[:])
	if err != nil {
		if errors.Is(notFound(err), identity.ErrNotFound) {
			return staff.Session{}, identity.ErrInvalidToken
		}
		return staff.Session{}, err
	}
	var h [32]byte
	if copy(h[:], r.TokenHash) != len(h) {
		return staff.Session{}, errors.New("session token hash has the wrong length")
	}
	return staff.Session{
		ID: r.ID, UserID: r.UserID, TokenHash: h, CreatedAt: r.CreatedAt.Time,
		LastSeenAt: r.LastSeenAt.Time, ExpiresAt: r.ExpiresAt.Time, MFAAt: timePtr(r.MfaAt),
		IP: r.Ip, UserAgent: r.UserAgent, RevokedAt: timePtr(r.RevokedAt),
	}, nil
}

// Touch records activity.
func (s *Store) Touch(ctx context.Context, id string, at time.Time) error {
	return sqlcgen.New(s.pool).TouchStaffSession(ctx, sqlcgen.TouchStaffSessionParams{
		ID: id, At: ts(at),
	})
}

// SetMFA records that a passkey was used.
func (s *Store) SetMFA(ctx context.Context, id string, at time.Time) error {
	n, err := sqlcgen.New(s.pool).SetStaffSessionMFA(ctx, sqlcgen.SetStaffSessionMFAParams{
		ID: id, At: ts(at),
	})
	if err == nil && n == 0 {
		return identity.ErrInvalidSession
	}
	return err
}

// RevokeSession ends one of the member's sessions.
func (s *Store) RevokeSession(ctx context.Context, userID, id string, at time.Time) error {
	_, err := sqlcgen.New(s.pool).RevokeStaffSession(ctx, sqlcgen.RevokeStaffSessionParams{
		ID: id, UserID: userID, At: ts(at),
	})
	return err
}

// SaveChallenge keeps a ceremony's state.
func (s *Store) SaveChallenge(ctx context.Context, c staff.Challenge) error {
	return sqlcgen.New(s.pool).SaveChallenge(ctx, sqlcgen.SaveChallengeParams{
		SessionID: c.SessionID, Kind: string(c.Kind), Data: c.Data, ExpiresAt: ts(c.ExpiresAt),
	})
}

// TakeChallenge returns and deletes a ceremony's state in one statement.
func (s *Store) TakeChallenge(
	ctx context.Context, sessionID string, kind staff.ChallengeKind, at time.Time,
) ([]byte, error) {
	data, err := sqlcgen.New(s.pool).TakeChallenge(ctx, sqlcgen.TakeChallengeParams{
		SessionID: sessionID, Kind: string(kind), At: ts(at),
	})
	return data, notFound(err)
}
