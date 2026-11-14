package pgstore

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/zellapay/zellapay/internal/platform/idempotency"
	"github.com/zellapay/zellapay/internal/platform/idempotency/pgstore/sqlcgen"
)

// Complete stores the final response. It returns ErrLockLost if the claim
// no longer belongs to this request.
func (s *Store) Complete(
	ctx context.Context, req idempotency.Request, resp idempotency.Response, _ time.Time,
) error {
	if resp.Status < 100 || resp.Status > 599 {
		return fmt.Errorf("idempotency: invalid status %d", resp.Status)
	}
	n, err := sqlcgen.New(s.pool).CompleteKey(ctx, sqlcgen.CompleteKeyParams{
		Scope:               req.Scope,
		Key:                 req.Key,
		Fingerprint:         req.Fingerprint,
		ResponseStatus:      pgtype.Int4{Int32: int32(resp.Status), Valid: true},
		ResponseContentType: pgtype.Text{String: resp.ContentType, Valid: true},
		ResponseBody:        resp.Body,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return idempotency.ErrLockLost
	}
	return nil
}

// Release removes an in-progress claim so the client can retry.
func (s *Store) Release(ctx context.Context, req idempotency.Request) error {
	_, err := sqlcgen.New(s.pool).ReleaseKey(ctx, sqlcgen.ReleaseKeyParams{
		Scope: req.Scope, Key: req.Key, Fingerprint: req.Fingerprint,
	})
	return err
}

// PurgeExpired deletes up to batch expired records and returns how many
// it removed. Run it from a scheduled job until it returns zero.
func (s *Store) PurgeExpired(ctx context.Context, now time.Time, batch int32) (int64, error) {
	return sqlcgen.New(s.pool).PurgeExpiredKeys(ctx, sqlcgen.PurgeExpiredKeysParams{
		Now: ts(now), BatchSize: batch,
	})
}
