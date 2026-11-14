package payouts

import (
	"context"
	"errors"
	"fmt"

	"github.com/zellapay/zellapay/internal/operators"
)

// confirm asks the operator for the authoritative state of an open payout
// and applies it. Callbacks are only hints; this is the truth.
func (s *Service) confirm(ctx context.Context, p Payout) (Payout, error) {
	if !p.Status.Open() || p.FailureCode == CodeAmountMismatch {
		return p, nil
	}
	a, ok := s.d.Router.Adapter(p.Operator)
	if !ok {
		return p, fmt.Errorf("%w: %q", ErrUnknownOperator, p.Operator)
	}
	st, err := a.Status(ctx, operators.Ref{OperatorRef: p.OperatorRef, OurRef: p.ID})
	switch {
	case errors.Is(err, operators.ErrNotFound):
		return s.unseen(ctx, p)
	case err != nil:
		return s.d.Repo.Update(ctx, p.ID, func(cur Payout) (Payout, Event, error) {
			return s.recheck(cur)
		})
	}
	return s.apply(ctx, p, st)
}

// unseen handles an operator with no record of the request. If we never got
// an operator reference the request never arrived, so no money left and the
// hold can be returned. If we did, something is inconsistent and a person
// must look.
func (s *Service) unseen(ctx context.Context, p Payout) (Payout, error) {
	if p.OperatorRef == "" {
		return s.release(ctx, p, codeNeverReceived, EventConfirmed)
	}
	s.d.Log.Error("operator has no record of an accepted payout", "payout_id", p.ID)
	return s.d.Repo.Update(ctx, p.ID, s.toAmbiguous(EventUnknown))
}

// apply moves p to the state the operator reports.
func (s *Service) apply(ctx context.Context, p Payout, st operators.Status) (Payout, error) {
	switch st.State {
	case operators.StateSucceeded:
		return s.succeed(ctx, p)
	case operators.StateFailed:
		return s.release(ctx, p, failureCode(st), EventConfirmed)
	case operators.StateExpired:
		return s.release(ctx, p, codeExpired, EventConfirmed)
	case operators.StatePending:
		return s.d.Repo.Update(ctx, p.ID, s.stillPending())
	}
	return s.d.Repo.Update(ctx, p.ID, func(cur Payout) (Payout, Event, error) {
		return s.recheck(cur)
	})
}

func failureCode(st operators.Status) string {
	if st.RawCode == "" {
		return "failed"
	}
	return st.RawCode
}

// stillPending returns an ambiguous payout to pending, or reschedules a
// pending one, flagging it once it has waited unusually long.
func (s *Service) stillPending() UpdateFunc {
	return func(p Payout) (Payout, Event, error) {
		if p.Status == StatusAmbiguous {
			next, ev, err := p.transition(StatusPending, EventConfirmed, "still_pending", s.now())
			return next.withCheck(s.now().Add(s.cfg.PendingCheckAfter), s.now()), ev, err
		}
		next, _, _ := s.recheck(p)
		if s.now().Sub(p.CreatedAt) > s.cfg.StaleAfter {
			s.d.Log.Warn("payout pending beyond the expected time", "payout_id", p.ID)
			ev := Event{Type: EventStalePending, From: p.Status, To: p.Status, At: s.now()}
			return next, ev, nil
		}
		return next, Event{}, nil
	}
}
