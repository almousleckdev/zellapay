package audit

import (
	"context"

	"github.com/zellapay/zellapay/internal/platform/clock"
)

// Service records and reads the audit log.
type Service struct {
	repo  Repository
	clock clock.Clock
}

// NewService returns a Service.
func NewService(repo Repository, c clock.Clock) *Service {
	return &Service{repo: repo, clock: c}
}

// Prepare validates e and stamps it with the current time. Code that must
// record an action in the same transaction as the action itself calls Prepare
// and then the store's AppendIn: if the transaction fails, the entry never
// existed, and if it commits, the entry cannot be missing.
func (s *Service) Prepare(e Entry) (Pending, error) {
	clean, err := normalize(e)
	if err != nil {
		return Pending{}, err
	}
	changeJSON, err := canonicalChange(clean.Change)
	if err != nil {
		return Pending{}, invalid("change")
	}
	return Pending{Entry: clean, At: StampTime(s.clock.Now()), ChangeJSON: changeJSON}, nil
}

// Record validates and appends e in its own transaction.
func (s *Service) Record(ctx context.Context, e Entry) (Record, error) {
	p, err := s.Prepare(e)
	if err != nil {
		return Record{}, err
	}
	return s.repo.Append(ctx, p)
}

// Head returns the newest entry's position and hash.
func (s *Service) Head(ctx context.Context) (Head, error) { return s.repo.Head(ctx) }

// List returns entries matching f, newest first.
func (s *Service) List(ctx context.Context, f Filter) ([]Record, error) {
	if f.Limit < 0 || f.Limit > 200 {
		return nil, invalid("limit")
	}
	if f.Limit == 0 {
		f.Limit = 50
	}
	return s.repo.List(ctx, f)
}
