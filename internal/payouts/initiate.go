package payouts

import (
	"context"
	"errors"

	"github.com/zellapay/zellapay/internal/operators"
)

// advance takes a created or held payout as far as it can go now: hold the
// funds, then ask the operator. Every step is idempotent, so concurrent or
// repeated calls are safe.
func (s *Service) advance(
	ctx context.Context, p Payout, a operators.Adapter, phone string,
) (Payout, error) {
	if p.Status == StatusCreated {
		if err := s.hold(ctx, p); errors.Is(err, ErrInsufficientFunds) {
			rejected, uerr := s.d.Repo.Update(ctx, p.ID, s.toRejected())
			if uerr != nil {
				return rejected, uerr
			}
			return rejected, ErrInsufficientFunds
		} else if err != nil {
			return p, err
		}
		held, err := s.d.Repo.Update(ctx, p.ID, only(StatusCreated, s.toHeld()))
		if err != nil {
			return held, err
		}
		p = held
	}
	return s.initiate(ctx, p, a, phone)
}

// initiate asks the operator to pay. The reference sent is the payout ID,
// so repeating the call can never pay twice.
func (s *Service) initiate(
	ctx context.Context, p Payout, a operators.Adapter, phone string,
) (Payout, error) {
	if p.Status != StatusHeld {
		return p, nil
	}
	ref, err := a.InitiatePayout(ctx, operators.PayoutRequest{
		Reference: p.ID, Phone: phone, Amount: p.Amount,
		Currency: p.Currency, Narration: p.Description,
	})
	switch {
	case err == nil:
		return s.d.Repo.Update(ctx, p.ID, only(StatusHeld, s.toPending(ref.OperatorRef)))
	case errors.Is(err, operators.ErrRejected):
		return s.release(ctx, p, codeRejected, EventRejected)
	default:
		// A timeout or any unclear error may still have reached the
		// operator, so the outcome is unknown and the funds stay held.
		return s.d.Repo.Update(ctx, p.ID, only(StatusHeld, s.toAmbiguous(EventUnknown)))
	}
}

// only applies fn to a payout still in the given state. Concurrent retries
// of one request all reach this point; the first wins and the rest change
// nothing.
func only(from Status, fn UpdateFunc) UpdateFunc {
	return func(cur Payout) (Payout, Event, error) {
		if cur.Status != from {
			return cur, Event{}, ErrNoChange
		}
		return fn(cur)
	}
}

func (s *Service) toRejected() UpdateFunc {
	return func(p Payout) (Payout, Event, error) {
		next, ev, err := p.transition(StatusRejected, EventRejected, CodeInsufficientFunds, s.now())
		next.FailureCode = CodeInsufficientFunds
		return next, ev, err
	}
}

func (s *Service) toHeld() UpdateFunc {
	return func(p Payout) (Payout, Event, error) {
		next, ev, err := p.transition(StatusHeld, EventHeld, "", s.now())
		return next.withCheck(s.now().Add(s.cfg.CreatedGrace), s.now()), ev, err
	}
}

func (s *Service) toPending(opRef string) UpdateFunc {
	return func(p Payout) (Payout, Event, error) {
		next, ev, err := p.transition(StatusPending, EventInitiated, p.Operator, s.now())
		if err != nil {
			return p, Event{}, err
		}
		next.OperatorRef = opRef
		return next.withCheck(s.now().Add(s.cfg.PendingCheckAfter), s.now()), ev, nil
	}
}

func (s *Service) toAmbiguous(evt string) UpdateFunc {
	return func(p Payout) (Payout, Event, error) {
		if p.Status == StatusAmbiguous {
			return s.recheck(p)
		}
		next, ev, err := p.transition(StatusAmbiguous, evt, p.Operator, s.now())
		if err != nil {
			return p, Event{}, err
		}
		return next.withCheck(s.now().Add(s.cfg.AmbiguousCheckAfter), s.now()), ev, nil
	}
}

// recheck keeps the status and schedules the next look. The empty event is
// not recorded.
func (s *Service) recheck(p Payout) (Payout, Event, error) {
	wait := s.cfg.PendingCheckAfter
	if p.Status == StatusAmbiguous {
		wait = s.cfg.AmbiguousCheckAfter
	}
	if s.now().Sub(p.CreatedAt) > s.cfg.StaleAfter {
		wait = s.cfg.StaleCheckAfter
	}
	return p.withCheck(s.now().Add(wait), s.now()), Event{}, nil
}
