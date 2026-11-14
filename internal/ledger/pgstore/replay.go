package pgstore

import (
	"bytes"
	"context"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/ledger/pgstore/sqlcgen"
)

// replay resolves a reused idempotency key: an identical journal is a
// harmless retry, anything else is a conflict.
func replay(
	ctx context.Context, q *sqlcgen.Queries, j ledger.Journal, fp []byte,
) (ledger.PostResult, error) {
	existing, err := q.GetJournalByKey(ctx, j.IdempotencyKey)
	if err != nil {
		return ledger.PostResult{}, err
	}
	if !bytes.Equal(existing.Fingerprint, fp) {
		return ledger.PostResult{}, ledger.ErrIdempotencyConflict
	}
	return ledger.PostResult{JournalID: existing.ID, Replayed: true}, nil
}
