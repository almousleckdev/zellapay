package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/identity/pgstore/sqlcgen"
	"github.com/zellapay/zellapay/internal/platform/pgconv"
)

var _ identity.SessionRepository = (*Store)(nil)

func sessionFromRow(r sqlcgen.Session) (identity.Session, error) {
	var hash [32]byte
	if copy(hash[:], r.TokenHash) != len(hash) {
		return identity.Session{}, errors.New("session token hash has the wrong length")
	}
	return identity.Session{
		ID: r.ID, UserID: r.UserID, TokenHash: hash, CreatedAt: r.CreatedAt.Time,
		LastSeenAt: r.LastSeenAt.Time, ExpiresAt: r.ExpiresAt.Time,
		MFAAt:     pgconv.TimePtr(r.MfaAt),
		Source:    identity.Source{IP: r.Ip, UserAgent: r.UserAgent},
		RevokedAt: pgconv.TimePtr(r.RevokedAt),
	}, nil
}

// CreateSession stores a new session.
func (s *Store) CreateSession(ctx context.Context, sess identity.Session) error {
	return sqlcgen.New(s.pool).InsertSession(ctx, sqlcgen.InsertSessionParams{
		ID: sess.ID, UserID: sess.UserID, TokenHash: sess.TokenHash[:],
		CreatedAt: ts(sess.CreatedAt), ExpiresAt: ts(sess.ExpiresAt),
		Ip: sess.Source.IP, UserAgent: sess.Source.UserAgent,
	})
}

// FindSession returns the session for a token hash.
func (s *Store) FindSession(ctx context.Context, hash [32]byte) (identity.Session, error) {
	r, err := sqlcgen.New(s.pool).GetSessionByHash(ctx, hash[:])
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.Session{}, identity.ErrInvalidToken
	}
	if err != nil {
		return identity.Session{}, err
	}
	return sessionFromRow(r)
}

// Touch records activity.
func (s *Store) Touch(ctx context.Context, id string, at time.Time) error {
	return sqlcgen.New(s.pool).TouchSession(ctx, sqlcgen.TouchSessionParams{ID: id, At: ts(at)})
}

// SetMFA records that the second factor was proven.
func (s *Store) SetMFA(ctx context.Context, id string, at time.Time) error {
	n, err := sqlcgen.New(s.pool).SetSessionMFA(ctx, sqlcgen.SetSessionMFAParams{
		ID: id, At: ts(at),
	})
	if err == nil && n == 0 {
		return identity.ErrInvalidSession
	}
	return err
}

// RevokeSession ends one open session of the user.
func (s *Store) RevokeSession(
	ctx context.Context, userID, id string, at time.Time,
) (bool, error) {
	n, err := sqlcgen.New(s.pool).RevokeSession(ctx, sqlcgen.RevokeSessionParams{
		ID: id, UserID: userID, At: ts(at),
	})
	return n > 0, err
}

// RevokeUserSessions ends the user's open sessions except keepID.
func (s *Store) RevokeUserSessions(
	ctx context.Context, userID, keepID string, at time.Time,
) (int, error) {
	n, err := sqlcgen.New(s.pool).RevokeUserSessions(ctx, sqlcgen.RevokeUserSessionsParams{
		UserID: userID, KeepID: keepID, At: ts(at),
	})
	return int(n), err
}

// ListSessions returns the user's open sessions.
func (s *Store) ListSessions(
	ctx context.Context, userID string, at time.Time,
) ([]identity.Session, error) {
	rows, err := sqlcgen.New(s.pool).ListOpenSessions(ctx, sqlcgen.ListOpenSessionsParams{
		UserID: userID, At: ts(at),
	})
	if err != nil {
		return nil, err
	}
	out := make([]identity.Session, 0, len(rows))
	for _, r := range rows {
		sess, err := sessionFromRow(r)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, nil
}
