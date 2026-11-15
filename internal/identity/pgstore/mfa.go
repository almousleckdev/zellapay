package pgstore

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/identity/pgstore/sqlcgen"
	"github.com/zellapay/zellapay/internal/platform/ids"
	"github.com/zellapay/zellapay/internal/platform/pgconv"
)

var _ identity.MFARepository = (*Store)(nil)

// SaveEnrolment stores an unconfirmed secret, replacing an earlier one.
func (s *Store) SaveEnrolment(ctx context.Context, c identity.MFACredential) error {
	n, err := sqlcgen.New(s.pool).UpsertMFACredential(ctx, sqlcgen.UpsertMFACredentialParams{
		UserID: c.UserID, SecretEnc: c.SecretEnc, CreatedAt: ts(c.CreatedAt),
	})
	if err == nil && n == 0 {
		return identity.ErrMFAAlreadyEnrolled
	}
	return err
}

// FindMFA loads the user's credential, or ErrNotFound.
func (s *Store) FindMFA(ctx context.Context, userID string) (identity.MFACredential, error) {
	r, err := sqlcgen.New(s.pool).GetMFACredential(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.MFACredential{}, identity.ErrNotFound
	}
	if err != nil {
		return identity.MFACredential{}, err
	}
	return identity.MFACredential{
		UserID: r.UserID, SecretEnc: r.SecretEnc, ConfirmedAt: pgconv.TimePtr(r.ConfirmedAt),
		LastStep: r.LastStep, CreatedAt: r.CreatedAt.Time,
	}, nil
}

// Confirm activates the credential and stores fresh recovery codes.
func (s *Store) Confirm(
	ctx context.Context, c identity.Confirmation, fx identity.Effects,
) error {
	return s.inTx(ctx, fx, func(q *sqlcgen.Queries) error {
		n, err := q.ConfirmMFACredential(ctx, sqlcgen.ConfirmMFACredentialParams{
			UserID: c.UserID, Step: c.Step, At: ts(c.At),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return identity.ErrMFAAlreadyEnrolled
		}
		if err := q.DeleteRecoveryCodes(ctx, c.UserID); err != nil {
			return err
		}
		for _, h := range c.CodeHashes {
			err := q.InsertRecoveryCode(ctx, sqlcgen.InsertRecoveryCodeParams{
				ID: ids.New("rcv_"), UserID: c.UserID, CodeHash: h[:], CreatedAt: ts(c.At),
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// AdvanceStep stores the newest accepted time step. It is a single
// conditional update, so two requests with the same code cannot both pass.
func (s *Store) AdvanceStep(ctx context.Context, userID string, step int64) (bool, error) {
	n, err := sqlcgen.New(s.pool).AdvanceMFAStep(ctx, sqlcgen.AdvanceMFAStepParams{
		UserID: userID, Step: step,
	})
	return n > 0, err
}

// UseRecoveryCode spends one code. The effects are stored, in the same
// transaction, only if it was valid, so a wrong guess leaves no audit entry.
func (s *Store) UseRecoveryCode(
	ctx context.Context, u identity.RecoveryUse, fx identity.Effects,
) (ok bool, left int, err error) {
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		n, err := q.UseRecoveryCode(ctx, sqlcgen.UseRecoveryCodeParams{
			UserID: u.UserID, CodeHash: u.Hash[:], At: ts(u.At),
		})
		if err != nil || n == 0 {
			return err
		}
		ok = true
		c, err := q.CountRecoveryCodes(ctx, u.UserID)
		if err != nil {
			return err
		}
		left = int(c)
		return s.queue(ctx, tx, fx)
	})
	if err != nil {
		return false, 0, err
	}
	return ok, left, nil
}
