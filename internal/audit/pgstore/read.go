package pgstore

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/audit/pgstore/sqlcgen"
	"github.com/zellapay/zellapay/internal/platform/pgconv"
)

// After returns entries with a higher sequence number, oldest first.
func (s *Store) After(ctx context.Context, seq int64, limit int) ([]audit.Record, error) {
	if limit < 1 || limit > maxBatch {
		limit = maxBatch
	}
	rows, err := sqlcgen.New(s.pool).ListAuditAfter(ctx, sqlcgen.ListAuditAfterParams{
		AfterSeq: seq, BatchSize: pgconv.Int32(limit),
	})
	return records(rows, err)
}

// List returns entries matching f, newest first.
func (s *Store) List(ctx context.Context, f audit.Filter) ([]audit.Record, error) {
	rows, err := sqlcgen.New(s.pool).ListAuditNewestFirst(ctx, sqlcgen.ListAuditNewestFirstParams{
		BeforeSeq: f.BeforeSeq, ActorKind: text(string(f.ActorKind)), ActorID: text(f.ActorID),
		TargetKind: text(f.TargetKind), TargetID: text(f.TargetID), Action: text(f.Action),
		BatchSize: pgconv.Int32(f.Limit),
	})
	return records(rows, err)
}

func records(rows []sqlcgen.AuditLog, err error) ([]audit.Record, error) {
	if err != nil {
		return nil, err
	}
	out := make([]audit.Record, len(rows))
	for i, r := range rows {
		if out[i], err = recordFrom(r); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Verification returns how far the chain has been checked.
func (s *Store) Verification(ctx context.Context) (audit.VerifyState, error) {
	v, err := sqlcgen.New(s.pool).GetAuditVerification(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return audit.VerifyState{}, nil
	}
	if err != nil {
		return audit.VerifyState{}, err
	}
	hash, err := toHash(v.VerifiedHash)
	return audit.VerifyState{
		VerifiedSeq: v.VerifiedSeq, VerifiedHash: hash,
		BrokenSeq: v.BrokenSeq, BrokenReason: v.BrokenReason,
	}, err
}

// SaveVerification records how far the chain has been checked.
func (s *Store) SaveVerification(ctx context.Context, st audit.VerifyState) error {
	return sqlcgen.New(s.pool).SaveAuditVerification(ctx, sqlcgen.SaveAuditVerificationParams{
		VerifiedSeq: st.VerifiedSeq, VerifiedHash: st.VerifiedHash[:], VerifiedAt: ts(st.At),
		BrokenSeq: st.BrokenSeq, BrokenReason: st.BrokenReason,
	})
}
