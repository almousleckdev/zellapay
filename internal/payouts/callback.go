package payouts

import (
	"context"
	"errors"
	"strings"

	"github.com/zellapay/zellapay/internal/callbacks"
	"github.com/zellapay/zellapay/internal/operators"
)

// Owns reports whether reference was issued by this module.
func (s *Service) Owns(reference string) bool {
	return strings.HasPrefix(reference, idPrefix)
}

// HandleEvent processes a verified operator callback. It is checked against
// the original request and then confirmed with the operator before any
// money is recorded.
func (s *Service) HandleEvent(
	ctx context.Context, operatorCode string, ev operators.CallbackEvent,
) error {
	p, err := s.d.Repo.Get(ctx, ev.Ref.OurRef)
	if errors.Is(err, ErrNotFound) {
		return callbacks.ErrUnknownReference
	}
	if err != nil {
		return err
	}
	if p.Operator != operatorCode {
		return callbacks.ErrUnknownReference
	}
	if ev.Amount != p.Amount || ev.Currency != p.Currency {
		return s.flagMismatch(ctx, p, ev)
	}
	_, err = s.confirm(ctx, p)
	return err
}

// flagMismatch holds a payout whose callback disagrees with the request.
// Funds stay held, nothing is released or captured, and no automatic check
// is scheduled: only a person can decide what the operator really sent.
func (s *Service) flagMismatch(ctx context.Context, p Payout, ev operators.CallbackEvent) error {
	s.d.Log.Error("callback does not match the payout",
		"payout_id", p.ID, "expected", p.Amount, "reported", ev.Amount)
	_, err := s.d.Repo.Update(ctx, p.ID, s.holdForReview)
	return err
}

func (s *Service) holdForReview(cur Payout) (Payout, Event, error) {
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
