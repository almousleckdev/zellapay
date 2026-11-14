package collections

import "context"

// SweepResult counts what one sweep did.
type SweepResult struct {
	Checked int // open collections examined
	Changed int // collections whose status changed
	Failed  int // collections that could not be examined this time
}

// Sweep examines open collections that are due for a check. It is safe to
// run from several workers: every step is idempotent.
func (s *Service) Sweep(ctx context.Context, limit int) (SweepResult, error) {
	due, err := s.d.Repo.Due(ctx, s.now(), limit)
	if err != nil {
		return SweepResult{}, err
	}
	var res SweepResult
	for _, c := range due {
		res.Checked++
		next, err := s.sweepOne(ctx, c)
		switch {
		case err != nil:
			res.Failed++
			s.d.Log.Error("sweep check failed", "collection_id", c.ID, "err", err)
		case next.Status != c.Status:
			res.Changed++
		}
	}
	return res, nil
}

// sweepOne checks a single collection. A request still stored as created
// after the grace period may have reached the operator before a crash, so
// its outcome is unknown until the operator says otherwise.
func (s *Service) sweepOne(ctx context.Context, c Collection) (Collection, error) {
	if c.Status == StatusCreated {
		var err error
		c, err = s.d.Repo.Update(ctx, c.ID, s.toAmbiguous(c.Operator, EventUnknown))
		if err != nil {
			return c, err
		}
	}
	return s.confirm(ctx, c)
}

// Get returns a collection by ID.
func (s *Service) Get(ctx context.Context, id string) (Collection, error) {
	return s.d.Repo.Get(ctx, id)
}
