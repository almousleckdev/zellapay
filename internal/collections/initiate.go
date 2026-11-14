package collections

import (
	"context"
	"errors"

	"github.com/zellapay/zellapay/internal/operators"
)

// initiate asks the operator to collect. The reference sent is the
// collection ID, so repeating the call can never create a second charge.
func (s *Service) initiate(
	ctx context.Context, c Collection, a operators.Adapter, phone string,
) (Collection, error) {
	ref, err := a.InitiateCollection(ctx, operators.CollectionRequest{
		Reference: c.ID, Phone: phone, Amount: c.Amount,
		Currency: c.Currency, Narration: c.Description,
	})
	switch {
	case err == nil:
		return s.d.Repo.Update(ctx, c.ID, fromCreated(s.toPending(a.Code(), ref.OperatorRef)))
	case errors.Is(err, operators.ErrRejected):
		fail := s.toFailed(a.Code(), "rejected", EventRejected)
		return s.d.Repo.Update(ctx, c.ID, fromCreated(fail))
	default:
		// A timeout or any unclear error may still have reached the
		// operator, so the outcome is unknown, never failed.
		return s.d.Repo.Update(ctx, c.ID, fromCreated(s.toAmbiguous(a.Code(), EventUnknown)))
	}
}

// fromCreated applies fn only to a collection that has not yet left the
// created state. Concurrent retries of one request all reach this point;
// the first wins and the rest see the result and change nothing.
func fromCreated(fn UpdateFunc) UpdateFunc {
	return func(cur Collection) (Collection, Event, error) {
		if cur.Status != StatusCreated {
			return cur, Event{}, ErrNoChange
		}
		return fn(cur)
	}
}

func (s *Service) toPending(operator, opRef string) UpdateFunc {
	return func(c Collection) (Collection, Event, error) {
		next, ev, err := c.transition(StatusPending, EventInitiated, operator, s.now())
		if err != nil {
			return c, Event{}, err
		}
		next.Operator, next.OperatorRef = operator, opRef
		return next.withCheck(s.now().Add(s.cfg.PendingCheckAfter), s.now()), ev, nil
	}
}

func (s *Service) toAmbiguous(operator, evt string) UpdateFunc {
	return func(c Collection) (Collection, Event, error) {
		if c.Status == StatusAmbiguous {
			return s.recheck(c)
		}
		next, ev, err := c.transition(StatusAmbiguous, evt, operator, s.now())
		if err != nil {
			return c, Event{}, err
		}
		if next.Operator == "" {
			next.Operator = operator
		}
		return next.withCheck(s.now().Add(s.cfg.AmbiguousCheckAfter), s.now()), ev, nil
	}
}

func (s *Service) toFailed(operator, code, evt string) UpdateFunc {
	return func(c Collection) (Collection, Event, error) {
		next, ev, err := c.transition(StatusFailed, evt, code, s.now())
		if err != nil {
			return c, Event{}, err
		}
		if next.Operator == "" {
			next.Operator = operator
		}
		next.FailureCode = code
		return next, ev, nil
	}
}

// recheck keeps the status and schedules the next look. The empty event
// is not recorded.
func (s *Service) recheck(c Collection) (Collection, Event, error) {
	wait := s.cfg.PendingCheckAfter
	if c.Status == StatusAmbiguous {
		wait = s.cfg.AmbiguousCheckAfter
	}
	if s.now().Sub(c.CreatedAt) > s.cfg.StaleAfter {
		wait = s.cfg.StaleCheckAfter
	}
	return c.withCheck(s.now().Add(wait), s.now()), Event{}, nil
}
