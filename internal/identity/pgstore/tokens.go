package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/identity/pgstore/sqlcgen"
)

// CreateEmailToken replaces any open token of the same purpose and stores t.
func (s *Store) CreateEmailToken(
	ctx context.Context, t identity.EmailToken, fx identity.Effects,
) error {
	return s.inTx(ctx, fx, func(q *sqlcgen.Queries) error {
		err := q.ReplaceEmailToken(ctx, sqlcgen.ReplaceEmailTokenParams{
			UserID: t.UserID, Purpose: string(t.Purpose),
		})
		if err != nil {
			return err
		}
		return q.InsertEmailToken(ctx, sqlcgen.InsertEmailTokenParams{
			ID: t.ID, UserID: t.UserID, Purpose: string(t.Purpose),
			TokenHash: t.TokenHash[:], ExpiresAt: ts(t.ExpiresAt), CreatedAt: ts(t.CreatedAt),
		})
	})
}

// FindEmailToken returns an unused token.
func (s *Store) FindEmailToken(
	ctx context.Context, p identity.Purpose, hash [32]byte,
) (identity.EmailToken, error) {
	r, err := sqlcgen.New(s.pool).GetEmailTokenByHash(ctx, sqlcgen.GetEmailTokenByHashParams{
		TokenHash: hash[:], Purpose: string(p),
	})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && r.UsedAt.Valid) {
		return identity.EmailToken{}, identity.ErrInvalidToken
	}
	if err != nil {
		return identity.EmailToken{}, err
	}
	return identity.EmailToken{
		ID: r.ID, UserID: r.UserID, Purpose: p, TokenHash: hash,
		ExpiresAt: r.ExpiresAt.Time, CreatedAt: r.CreatedAt.Time,
	}, nil
}

// UseEmailToken consumes the token and applies what it allows: a new
// password, and a verified email either way the inbox was proven.
func (s *Store) UseEmailToken(
	ctx context.Context, use identity.TokenUse, fx identity.Effects,
) error {
	return s.inTx(ctx, fx, func(q *sqlcgen.Queries) error {
		_, err := q.UseEmailToken(ctx, sqlcgen.UseEmailTokenParams{
			TokenHash: use.TokenHash[:], Purpose: string(use.Purpose), At: ts(use.At),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return identity.ErrInvalidToken
		}
		if err != nil {
			return err
		}
		if use.NewPasswordHash != "" {
			if err := setPassword(ctx, q, passwordSet{
				UserID: use.UserID, Hash: use.NewPasswordHash, At: use.At,
			}); err != nil {
				return err
			}
		}
		return q.MarkEmailVerified(ctx, sqlcgen.MarkEmailVerifiedParams{
			ID: use.UserID, At: ts(use.At),
		})
	})
}

// passwordSet is a new password and the one session allowed to survive it.
type passwordSet struct {
	UserID, Hash, KeepSessionID string
	At                          time.Time
}

// setPassword stores the hash and ends every session except the kept one:
// whoever held another, including a thief, is signed out.
func setPassword(ctx context.Context, q *sqlcgen.Queries, p passwordSet) error {
	err := q.SetPassword(ctx, sqlcgen.SetPasswordParams{
		ID: p.UserID, PasswordHash: p.Hash, ChangedAt: ts(p.At),
	})
	if err != nil {
		return err
	}
	_, err = q.RevokeUserSessions(ctx, sqlcgen.RevokeUserSessionsParams{
		UserID: p.UserID, KeepID: p.KeepSessionID, At: ts(p.At),
	})
	return err
}
