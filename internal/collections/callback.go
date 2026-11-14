package collections

import (
	"context"
	"errors"
	"net/http"

	"github.com/zellapay/zellapay/internal/operators"
)

// HandleCallback processes a notification from an operator. The callback is
// verified by the adapter, checked against the original request, and then
// confirmed with the operator before any money is recorded.
func (s *Service) HandleCallback(ctx context.Context, operatorCode string, r *http.Request) error {
	a, ok := s.d.Router.Adapter(operatorCode)
	if !ok {
		return ErrUnknownOperator
	}
	ev, err := a.ParseCallback(ctx, r)
	if err != nil {
		return err
	}
	c, err := s.d.Repo.Get(ctx, ev.Ref.OurRef)
	if errors.Is(err, ErrNotFound) {
		return ErrUnknownReference
	}
	if err != nil {
		return err
	}
	if c.Operator != operatorCode {
		return ErrUnknownReference
	}
	if ev.Amount != c.Amount || ev.Currency != c.Currency {
		return s.flagMismatch(ctx, c, ev)
	}
	_, err = s.confirm(ctx, c)
	return err
}

// flagMismatch holds a collection whose callback disagrees with the request.
// Nothing is posted and no further automatic check is scheduled: only a
// person can decide what the operator really charged.
func (s *Service) flagMismatch(
	ctx context.Context, c Collection, ev operators.CallbackEvent,
) error {
	s.d.Log.Error("callback does not match the collection",
		"collection_id", c.ID, "expected", c.Amount, "reported", ev.Amount)
	_, err := s.d.Repo.Update(ctx, c.ID, s.holdForReview)
	return err
}

// holdForReview moves an open collection to ambiguous and marks it for a
// person. Holding an already held collection changes nothing.
func (s *Service) holdForReview(cur Collection) (Collection, Event, error) {
	if !cur.Status.Open() || cur.FailureCode == CodeAmountMismatch {
		return cur, Event{}, ErrNoChange
	}
	next := cur
	evt := Event{Type: EventAmountMismatch, From: cur.Status, To: StatusAmbiguous, At: s.now()}
	if cur.Status != StatusAmbiguous {
		var err error
		next, evt, err = cur.transition(StatusAmbiguous, EventAmountMismatch, "", s.now())
		if err != nil {
			return cur, Event{}, err
		}
	}
	next.FailureCode = CodeAmountMismatch
	next.NextCheckAt = nil
	return next, evt, nil
}
