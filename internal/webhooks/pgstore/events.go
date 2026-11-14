package pgstore

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/zellapay/zellapay/internal/webhooks"
	"github.com/zellapay/zellapay/internal/webhooks/pgstore/sqlcgen"
)

// SaveEvent stores ev. If an event for the same type and object exists it is
// returned instead, so publishing the same outbox entry twice is harmless.
func (s *Store) SaveEvent(ctx context.Context, ev webhooks.Event) (webhooks.Event, error) {
	q := sqlcgen.New(s.pool)
	_, err := q.InsertEvent(ctx, sqlcgen.InsertEventParams{
		ID: ev.ID, MerchantID: ev.MerchantID, Type: ev.Type, ObjectID: ev.ObjectID,
		Payload: ev.Payload, CreatedAt: ts(ev.CreatedAt),
	})
	if err == nil {
		return ev, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return webhooks.Event{}, err
	}
	row, err := q.GetEventByObject(ctx, sqlcgen.GetEventByObjectParams{
		Type: ev.Type, ObjectID: ev.ObjectID,
	})
	if err != nil {
		return webhooks.Event{}, err
	}
	return eventFrom(row), nil
}

// GetEvent returns an event that belongs to merchantID.
func (s *Store) GetEvent(ctx context.Context, merchantID, eventID string) (webhooks.Event, error) {
	row, err := sqlcgen.New(s.pool).GetEvent(ctx, sqlcgen.GetEventParams{
		ID: eventID, MerchantID: merchantID,
	})
	if err != nil {
		return webhooks.Event{}, notFound(err)
	}
	return eventFrom(row), nil
}
