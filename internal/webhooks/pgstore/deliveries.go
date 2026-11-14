package pgstore

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/zellapay/zellapay/internal/platform/pgconv"
	"github.com/zellapay/zellapay/internal/webhooks"
	"github.com/zellapay/zellapay/internal/webhooks/pgstore/sqlcgen"
)

const maxBatch = 500

// Enqueue schedules a delivery once per event and endpoint.
func (s *Store) Enqueue(
	ctx context.Context, eventID, endpointID string, sched webhooks.Schedule,
) error {
	return sqlcgen.New(s.pool).EnqueueDelivery(ctx, sqlcgen.EnqueueDeliveryParams{
		EventID: eventID, EndpointID: endpointID, At: ts(sched.At), RetryUntil: ts(sched.Until),
	})
}

// ClaimDue returns due deliveries and pushes their next attempt forward by
// lease, so a second worker cannot send the same delivery while the first is
// sending. If a worker dies, the lease expires and the delivery is retried.
func (s *Store) ClaimDue(
	ctx context.Context, now time.Time, limit int, lease time.Duration,
) ([]webhooks.Work, error) {
	if limit < 1 || limit > maxBatch {
		limit = maxBatch
	}
	q := sqlcgen.New(s.pool)
	ids, err := q.ClaimDueDeliveries(ctx, sqlcgen.ClaimDueDeliveriesParams{
		Now: ts(now), BatchSize: int32(limit), LeaseUntil: ts(now.Add(lease)),
	})
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	rows, err := q.GetDeliveryWork(ctx, ids)
	if err != nil {
		return nil, err
	}
	work := make([]webhooks.Work, len(rows))
	for i, r := range rows {
		work[i] = workFrom(r)
	}
	return work, nil
}

// Record stores an attempt and the delivery's new state atomically.
func (s *Store) Record(
	ctx context.Context, deliveryID int64, a webhooks.Attempt, o webhooks.Outcome,
) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		if err := q.InsertAttempt(ctx, sqlcgen.InsertAttemptParams{
			DeliveryID: deliveryID, Attempt: pgconv.Int32(a.Number), At: ts(a.At),
			StatusCode: pgconv.Int32(a.StatusCode), Error: a.Error,
			DurationMs: pgconv.Int32(int(a.Duration.Milliseconds())),
		}); err != nil {
			return err
		}
		return q.UpdateDelivery(ctx, sqlcgen.UpdateDeliveryParams{
			ID: deliveryID, Status: string(o.Status), Attempts: pgconv.Int32(a.Number),
			NextAttemptAt: optTS(o.NextAttemptAt), LastStatus: pgconv.Int32(a.StatusCode),
			LastError: a.Error, CompletedAt: optTS(o.CompletedAt),
		})
	})
}

// Rearm makes every delivery of an event pending again, for replay.
func (s *Store) Rearm(ctx context.Context, eventID string, now, until time.Time) (int, error) {
	n, err := sqlcgen.New(s.pool).RearmDeliveries(ctx, sqlcgen.RearmDeliveriesParams{
		EventID: eventID, Now: ts(now), RetryUntil: ts(until),
	})
	return int(n), err
}

// Deliveries lists an event's deliveries.
func (s *Store) Deliveries(ctx context.Context, eventID string) ([]webhooks.Delivery, error) {
	rows, err := sqlcgen.New(s.pool).ListDeliveries(ctx, eventID)
	if err != nil {
		return nil, err
	}
	out := make([]webhooks.Delivery, len(rows))
	for i, r := range rows {
		out[i] = webhooks.Delivery{
			ID: r.ID, EventID: r.EventID, EndpointID: r.EndpointID,
			Status: webhooks.DeliveryStatus(r.Status), Attempts: int(r.Attempts),
			NextAttemptAt: timePtr(r.NextAttemptAt), RetryUntil: r.RetryUntil.Time,
			LastStatus: int(r.LastStatus), LastError: r.LastError,
			CreatedAt: r.CreatedAt.Time, CompletedAt: timePtr(r.CompletedAt),
		}
	}
	return out, nil
}

// Attempts returns the delivery log of one delivery, oldest first.
func (s *Store) Attempts(ctx context.Context, deliveryID int64) ([]webhooks.Attempt, error) {
	rows, err := sqlcgen.New(s.pool).ListAttempts(ctx, deliveryID)
	if err != nil {
		return nil, err
	}
	out := make([]webhooks.Attempt, len(rows))
	for i, r := range rows {
		out[i] = webhooks.Attempt{
			Number: int(r.Attempt), At: r.At.Time, StatusCode: int(r.StatusCode),
			Error: r.Error, Duration: time.Duration(r.DurationMs) * time.Millisecond,
		}
	}
	return out, nil
}
