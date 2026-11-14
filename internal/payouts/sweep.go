package payouts

import (
	"context"
	"time"
)

// SweepResult counts what one sweep did.
type SweepResult struct {
	Checked int // payouts examined
	Changed int // payouts whose status changed
	Failed  int // payouts that could not be examined this time
}

// Sweep examines payouts that are due for a check. It is safe to run from
// several workers: every step is idempotent.
func (s *Service) Sweep(ctx context.Context, limit int) (SweepResult, error) {
	due, err := s.d.Repo.Due(ctx, s.now(), limit)
	if err != nil {
		return SweepResult{}, err
	}
	var res SweepResult
	for _, p := range due {
		res.Checked++
		next, err := s.sweepOne(ctx, p)
		switch {
		case err != nil:
			res.Failed++
			s.d.Log.Error("payout sweep failed", "payout_id", p.ID, "err", err)
		case next.Status != p.Status:
			res.Changed++
		}
	}
	return res, nil
}

// sweepOne checks one payout. A payout stored but never held was never sent,
// so it is ended safely. One held but not yet confirmed may have reached the
// operator before a crash, so its outcome is unknown until the operator says.
func (s *Service) sweepOne(ctx context.Context, p Payout) (Payout, error) {
	switch p.Status {
	case StatusCreated:
		return s.abandon(ctx, p)
	case StatusHeld:
		toAmbiguous := only(StatusHeld, s.toAmbiguous(EventUnknown))
		var err error
		if p, err = s.d.Repo.Update(ctx, p.ID, toAmbiguous); err != nil {
			return p, err
		}
	}
	return s.confirm(ctx, p)
}

// ClaimOutbox offers unpublished outbox entries to handle. See
// Repository.ClaimOutbox.
func (s *Service) ClaimOutbox(
	ctx context.Context, now time.Time, limit int, handle OutboxHandler,
) (int, error) {
	return s.d.Repo.ClaimOutbox(ctx, now, limit, handle)
}
