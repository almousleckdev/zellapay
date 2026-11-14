package collections

import (
	"context"
	"errors"
	"fmt"

	"github.com/zellapay/zellapay/internal/operators"
)

// confirm asks the operator for the authoritative state of an open
// collection and applies it. Callbacks are only hints; this is the truth.
func (s *Service) confirm(ctx context.Context, c Collection) (Collection, error) {
	if !c.Status.Open() || c.FailureCode == CodeAmountMismatch {
		return c, nil
	}
	a, ok := s.d.Router.Adapter(c.Operator)
	if !ok {
		return c, fmt.Errorf("%w: %q", ErrUnknownOperator, c.Operator)
	}
	st, err := a.Status(ctx, operators.Ref{OperatorRef: c.OperatorRef, OurRef: c.ID})
	switch {
	case errors.Is(err, operators.ErrNotFound):
		return s.unseen(ctx, c)
	case err != nil:
		return s.d.Repo.Update(ctx, c.ID, func(cur Collection) (Collection, Event, error) {
			return s.recheck(cur)
		})
	}
	return s.apply(ctx, c, st)
}

// unseen handles an operator with no record of the request. If we never
// received an operator reference the request never arrived, so it is safe to
// fail. If we did, something is inconsistent and a person must look.
func (s *Service) unseen(ctx context.Context, c Collection) (Collection, error) {
	if c.OperatorRef == "" {
		fail := s.toFailed(c.Operator, "operator_never_received", EventConfirmed)
		return s.d.Repo.Update(ctx, c.ID, fail)
	}
	s.d.Log.Error("operator has no record of an accepted collection", "collection_id", c.ID)
	return s.d.Repo.Update(ctx, c.ID, s.toAmbiguous(c.Operator, EventUnknown))
}

// apply moves c to the state the operator reports.
func (s *Service) apply(
	ctx context.Context, c Collection, st operators.Status,
) (Collection, error) {
	switch st.State {
	case operators.StateSucceeded:
		return s.succeed(ctx, c)
	case operators.StateFailed:
		return s.d.Repo.Update(ctx, c.ID, s.toFailed(c.Operator, failureCode(st), EventConfirmed))
	case operators.StateExpired:
		return s.d.Repo.Update(ctx, c.ID, s.toExpired())
	case operators.StatePending:
		return s.d.Repo.Update(ctx, c.ID, s.stillPending())
	}
	return s.d.Repo.Update(ctx, c.ID, func(cur Collection) (Collection, Event, error) {
		return s.recheck(cur)
	})
}

func failureCode(st operators.Status) string {
	if st.RawCode == "" {
		return "failed"
	}
	return st.RawCode
}

func (s *Service) toExpired() UpdateFunc {
	return func(c Collection) (Collection, Event, error) {
		return c.transition(StatusExpired, EventConfirmed, "operator_timeout", s.now())
	}
}

// stillPending returns an ambiguous collection to pending, or reschedules a
// pending one, flagging it once it has waited unusually long.
func (s *Service) stillPending() UpdateFunc {
	return func(c Collection) (Collection, Event, error) {
		if c.Status == StatusAmbiguous {
			next, ev, err := c.transition(StatusPending, EventConfirmed, "still_pending", s.now())
			return next.withCheck(s.now().Add(s.cfg.PendingCheckAfter), s.now()), ev, err
		}
		next, _, _ := s.recheck(c)
		if s.now().Sub(c.CreatedAt) > s.cfg.StaleAfter {
			s.d.Log.Warn("collection pending beyond the expected time", "collection_id", c.ID)
			ev := Event{Type: EventStalePending, From: c.Status, To: c.Status, At: s.now()}
			return next, ev, nil
		}
		return next, Event{}, nil
	}
}

// succeed records the money, then marks the collection succeeded. A
// collection already in a different final state is a contradiction: record
// it for a person and do not post.
func (s *Service) succeed(ctx context.Context, c Collection) (Collection, error) {
	fresh, err := s.d.Repo.Get(ctx, c.ID)
	if err != nil {
		return c, err
	}
	if fresh.Status == StatusSucceeded {
		return fresh, nil
	}
	if fresh.Status.Terminal() {
		return fresh, s.contradiction(ctx, fresh, StatusSucceeded)
	}
	if err := s.postToLedger(ctx, fresh); err != nil {
		return fresh, err
	}
	return s.d.Repo.Update(ctx, c.ID, func(cur Collection) (Collection, Event, error) {
		return cur.transition(StatusSucceeded, EventConfirmed, "operator_succeeded", s.now())
	})
}

func (s *Service) contradiction(ctx context.Context, c Collection, reported Status) error {
	s.d.Log.Error("operator contradicts a final collection",
		"collection_id", c.ID, "final", c.Status, "reported", reported)
	return s.d.Repo.Record(ctx, c.ID, Event{
		Type: EventContradiction, From: c.Status, To: reported,
		Detail: "operator reported " + string(reported), At: s.now(),
	})
}
