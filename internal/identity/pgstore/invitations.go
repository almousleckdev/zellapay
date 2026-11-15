package pgstore

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/identity/pgstore/sqlcgen"
)

// CreateInvitation revokes open invitations for the same person and merchant,
// stores inv and queues its effects.
func (s *Store) CreateInvitation(
	ctx context.Context, inv identity.Invitation, fx identity.Effects,
) error {
	return s.inTx(ctx, fx, func(q *sqlcgen.Queries) error {
		err := q.RevokeOpenInvitations(ctx, sqlcgen.RevokeOpenInvitationsParams{
			MerchantID: inv.MerchantID, Email: inv.Email, At: ts(inv.CreatedAt),
		})
		if err != nil {
			return err
		}
		return q.InsertInvitation(ctx, sqlcgen.InsertInvitationParams{
			ID: inv.ID, MerchantID: inv.MerchantID, Email: inv.Email,
			Role: string(inv.Role), Locale: inv.Locale, TokenHash: inv.TokenHash[:],
			InvitedBy: inv.InvitedBy, ExpiresAt: ts(inv.ExpiresAt), CreatedAt: ts(inv.CreatedAt),
		})
	})
}

// FindInvitation returns an open invitation.
func (s *Store) FindInvitation(
	ctx context.Context, hash [32]byte,
) (identity.Invitation, error) {
	r, err := sqlcgen.New(s.pool).GetInvitationByHash(ctx, hash[:])
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.Invitation{}, identity.ErrInvalidToken
	}
	if err != nil {
		return identity.Invitation{}, err
	}
	return identity.Invitation{
		ID: r.ID, MerchantID: r.MerchantID, Email: r.Email, Role: identity.Role(r.Role),
		Locale: r.Locale, TokenHash: hash, InvitedBy: r.InvitedBy,
		ExpiresAt: r.ExpiresAt.Time, CreatedAt: r.CreatedAt.Time,
	}, nil
}

// AcceptInvitation uses the invitation and creates the user in one
// transaction. The use is a conditional update, so it succeeds only once.
func (s *Store) AcceptInvitation(
	ctx context.Context, a identity.Acceptance, fx identity.Effects,
) error {
	err := s.inTx(ctx, fx, func(q *sqlcgen.Queries) error {
		_, err := q.UseInvitation(ctx, sqlcgen.UseInvitationParams{
			TokenHash: a.TokenHash[:], At: ts(a.At),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return identity.ErrInvalidToken
		}
		if err != nil {
			return err
		}
		return insertUser(ctx, q, a.User)
	})
	if isUnique(err) {
		return identity.ErrEmailTaken
	}
	return err
}
