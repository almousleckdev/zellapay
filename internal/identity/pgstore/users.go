package pgstore

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/identity/pgstore/sqlcgen"
	"github.com/zellapay/zellapay/internal/platform/pgconv"
)

func userFromRow(r sqlcgen.MerchantUser) identity.User {
	return identity.User{
		ID: r.ID, MerchantID: r.MerchantID, Email: r.Email, Name: r.Name,
		Role: identity.Role(r.Role), Status: identity.Status(r.Status), Locale: r.Locale,
		PasswordHash: r.PasswordHash, PasswordChangedAt: r.PasswordChangedAt.Time,
		EmailVerifiedAt: pgconv.TimePtr(r.EmailVerifiedAt), CreatedAt: r.CreatedAt.Time,
	}
}

func insertUser(ctx context.Context, q *sqlcgen.Queries, u identity.User) error {
	return q.InsertUser(ctx, sqlcgen.InsertUserParams{
		ID: u.ID, MerchantID: u.MerchantID, Email: u.Email, Name: u.Name,
		Role: string(u.Role), Status: string(u.Status), Locale: u.Locale,
		PasswordHash: u.PasswordHash, PasswordChangedAt: ts(u.PasswordChangedAt),
		EmailVerifiedAt: pgconv.OptTimestamptz(u.EmailVerifiedAt), CreatedAt: ts(u.CreatedAt),
	})
}

// FindUserByEmail loads a user by normalised email.
func (s *Store) FindUserByEmail(ctx context.Context, email string) (identity.User, error) {
	r, err := sqlcgen.New(s.pool).GetUserByEmail(ctx, email)
	return oneUser(r, err)
}

// FindUser loads a user by ID.
func (s *Store) FindUser(ctx context.Context, id string) (identity.User, error) {
	r, err := sqlcgen.New(s.pool).GetUser(ctx, id)
	return oneUser(r, err)
}

func oneUser(r sqlcgen.MerchantUser, err error) (identity.User, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.User{}, identity.ErrNotFound
	}
	if err != nil {
		return identity.User{}, err
	}
	return userFromRow(r), nil
}

// ChangePassword stores a new hash and ends the user's other sessions.
func (s *Store) ChangePassword(
	ctx context.Context, c identity.PasswordChange, fx identity.Effects,
) error {
	return s.inTx(ctx, fx, func(q *sqlcgen.Queries) error {
		return setPassword(ctx, q, passwordSet{
			UserID: c.UserID, Hash: c.PasswordHash, KeepSessionID: c.KeepSessionID, At: c.At,
		})
	})
}

// Rehash replaces oldHash with newHash if oldHash is still current.
func (s *Store) Rehash(ctx context.Context, id, oldHash, newHash string) error {
	return sqlcgen.New(s.pool).Rehash(ctx, sqlcgen.RehashParams{
		ID: id, OldHash: oldHash, PasswordHash: newHash,
	})
}
