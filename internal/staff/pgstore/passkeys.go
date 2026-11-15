package pgstore

import (
	"context"
	"time"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/staff"
	"github.com/zellapay/zellapay/internal/staff/pgstore/sqlcgen"
)

// AddPasskey stores a registered passkey and its audit entries.
func (s *Store) AddPasskey(ctx context.Context, p staff.Passkey, fx staff.Effects) error {
	err := s.inTx(ctx, fx, func(q *sqlcgen.Queries) error {
		return q.InsertCredential(ctx, sqlcgen.InsertCredentialParams{
			ID: p.ID, UserID: p.UserID, CredentialID: p.CredentialID, Label: p.Label,
			Data: p.Data, CreatedAt: ts(p.CreatedAt),
		})
	})
	if isUnique(err) {
		return identity.ErrInvalidCode
	}
	return err
}

// Passkeys lists a member's passkeys, oldest first.
func (s *Store) Passkeys(ctx context.Context, userID string) ([]staff.Passkey, error) {
	rows, err := sqlcgen.New(s.pool).ListCredentials(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]staff.Passkey, 0, len(rows))
	for _, r := range rows {
		out = append(out, staff.Passkey{
			ID: r.ID, UserID: r.UserID, CredentialID: r.CredentialID, Label: r.Label,
			Data: r.Data, CreatedAt: r.CreatedAt.Time, LastUsedAt: timePtr(r.LastUsedAt),
		})
	}
	return out, nil
}

// UpdatePasskey stores the credential's new counter after it was used.
func (s *Store) UpdatePasskey(ctx context.Context, p staff.Passkey, at time.Time) error {
	return sqlcgen.New(s.pool).UpdateCredential(ctx, sqlcgen.UpdateCredentialParams{
		CredentialID: p.CredentialID, UserID: p.UserID, Data: p.Data, At: ts(at),
	})
}
