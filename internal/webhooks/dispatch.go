package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zellapay/zellapay/internal/platform/ids"
)

// Dispatch turns outbox entries into events and schedules their deliveries.
// It is idempotent: an entry offered twice produces one event.
func (s *Service) Dispatch(ctx context.Context, limit int) (int, error) {
	return s.d.Feed.Claim(ctx, s.now(), limit, s.publishAll)
}

func (s *Service) publishAll(ctx context.Context, entries []OutboxEntry) error {
	for _, e := range entries {
		if err := s.publish(ctx, e); err != nil {
			return fmt.Errorf("publish %s: %w", e.Type, err)
		}
	}
	return nil
}

func (s *Service) publish(ctx context.Context, e OutboxEntry) error {
	merchantID, object, err := s.d.Source.Object(ctx, e)
	if errors.Is(err, ErrNotFound) {
		// The object is gone, so there is nothing to announce. Skipping keeps
		// one bad entry from blocking everything behind it.
		s.d.Log.Error("outbox entry has no object", "object_id", e.ObjectID, "type", e.Type)
		return nil
	}
	if err != nil {
		return err
	}
	ev, err := s.newEvent(e, merchantID, object)
	if err != nil {
		return err
	}
	saved, err := s.d.Repo.SaveEvent(ctx, ev)
	if err != nil {
		return err
	}
	return s.fanOut(ctx, saved)
}

func (s *Service) newEvent(
	e OutboxEntry, merchantID string, object json.RawMessage,
) (Event, error) {
	id := ids.New("evt_")
	env := envelope{ID: id, Type: e.Type, CreatedAt: e.CreatedAt.UTC()}
	env.Data.Object = object
	payload, err := json.Marshal(env)
	if err != nil {
		return Event{}, err
	}
	return Event{
		ID: id, MerchantID: merchantID, Type: e.Type, ObjectID: e.ObjectID,
		Payload: payload, CreatedAt: e.CreatedAt,
	}, nil
}

// fanOut schedules the event for every subscribed, active endpoint.
func (s *Service) fanOut(ctx context.Context, ev Event) error {
	endpoints, err := s.d.Repo.EndpointsFor(ctx, ev.MerchantID, ev.Type)
	if err != nil {
		return err
	}
	now := s.now()
	sched := Schedule{At: now, Until: now.Add(RetryHorizon)}
	for _, ep := range endpoints {
		if err := s.d.Repo.Enqueue(ctx, ev.ID, ep.ID, sched); err != nil {
			return err
		}
	}
	return nil
}

// Replay sends an event again to the merchant's current endpoints. It
// returns how many deliveries were scheduled.
func (s *Service) Replay(ctx context.Context, merchantID, eventID string) (int, error) {
	ev, err := s.d.Repo.GetEvent(ctx, merchantID, eventID)
	if err != nil {
		return 0, err
	}
	if err := s.fanOut(ctx, ev); err != nil {
		return 0, err
	}
	now := s.now()
	return s.d.Repo.Rearm(ctx, ev.ID, now, now.Add(RetryHorizon))
}
