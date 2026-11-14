package payouts

import (
	"context"
	"errors"

	"github.com/zellapay/zellapay/internal/ledger"
)

// succeed records that the money left, then marks the payout succeeded. The
// ledger comes first: a crash can leave the money recorded with the status
// still open, which the next check repairs, never a succeeded status
// without its journal.
func (s *Service) succeed(ctx context.Context, p Payout) (Payout, error) {
	fresh, done, err := s.fresh(ctx, p, StatusSucceeded)
	if done || err != nil {
		return fresh, err
	}
	if err := s.ensureAccounts(ctx, fresh); err != nil {
		return fresh, err
	}
	if err := s.post(ctx, fresh.captureJournal()); err != nil {
		return fresh, s.resolutionError(ctx, fresh, StatusSucceeded, err)
	}
	return s.d.Repo.Update(ctx, p.ID, func(cur Payout) (Payout, Event, error) {
		return cur.transition(StatusSucceeded, EventConfirmed, "operator_succeeded", s.now())
	})
}

// release returns the held funds and marks the payout failed.
func (s *Service) release(ctx context.Context, p Payout, code, evt string) (Payout, error) {
	fresh, done, err := s.fresh(ctx, p, StatusFailed)
	if done || err != nil {
		return fresh, err
	}
	if err := s.ensureAccounts(ctx, fresh); err != nil {
		return fresh, err
	}
	if err := s.post(ctx, fresh.releaseJournal()); err != nil {
		return fresh, s.resolutionError(ctx, fresh, StatusFailed, err)
	}
	return s.d.Repo.Update(ctx, p.ID, s.toFailed(code, evt))
}

func (s *Service) toFailed(code, evt string) UpdateFunc {
	return func(p Payout) (Payout, Event, error) {
		next, ev, err := p.transition(StatusFailed, evt, code, s.now())
		next.FailureCode = code
		return next, ev, err
	}
}

// fresh reloads the payout. done is true when there is nothing left to do:
// it already reached want, or it is final in a different state, which is
// recorded as a contradiction for a person to examine.
func (s *Service) fresh(ctx context.Context, p Payout, want Status) (Payout, bool, error) {
	cur, err := s.d.Repo.Get(ctx, p.ID)
	if err != nil {
		return p, true, err
	}
	switch {
	case cur.Status == want:
		return cur, true, nil
	case cur.Status.Terminal():
		return cur, true, s.contradiction(ctx, cur, want)
	}
	return cur, false, nil
}

// resolutionError turns the ledger's refusal to settle a payout a second,
// different way into a recorded contradiction.
func (s *Service) resolutionError(ctx context.Context, p Payout, reported Status, err error) error {
	if errors.Is(err, ledger.ErrIdempotencyConflict) {
		return s.contradiction(ctx, p, reported)
	}
	return err
}

func (s *Service) contradiction(ctx context.Context, p Payout, reported Status) error {
	s.d.Log.Error("operator contradicts a settled payout",
		"payout_id", p.ID, "final", p.Status, "reported", reported)
	return s.d.Repo.Record(ctx, p.ID, Event{
		Type: EventContradiction, From: p.Status, To: reported,
		Detail: "operator reported " + string(reported), At: s.now(),
	})
}

// abandon ends a payout that was stored but never sent. The hold is made
// first (it is idempotent), so the release below always has funds to return.
func (s *Service) abandon(ctx context.Context, p Payout) (Payout, error) {
	err := s.hold(ctx, p)
	if errors.Is(err, ErrInsufficientFunds) {
		return s.d.Repo.Update(ctx, p.ID, s.toRejected())
	}
	if err != nil {
		return p, err
	}
	return s.release(ctx, p, codeAbandoned, EventAbandoned)
}
