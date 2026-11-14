package collections

import (
	"context"
	"time"
)

// ClaimOutbox offers unpublished outbox entries to handle. See
// Repository.ClaimOutbox.
func (s *Service) ClaimOutbox(
	ctx context.Context, now time.Time, limit int, handle OutboxHandler,
) (int, error) {
	return s.d.Repo.ClaimOutbox(ctx, now, limit, handle)
}
