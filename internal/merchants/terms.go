package merchants

import "context"

const maxFeeBPS = 10_000

// FeeBPS returns the merchant's fee in hundredths of a percent.
func (s *Service) FeeBPS(ctx context.Context, merchantID string) (int, error) {
	return s.repo.FeeBPS(ctx, merchantID)
}

// SetFeeBPS sets the merchant's fee in hundredths of a percent, from 0 to
// 10000. A new merchant starts at 0 so nobody is charged by accident.
func (s *Service) SetFeeBPS(ctx context.Context, merchantID string, bps int) error {
	if bps < 0 || bps > maxFeeBPS {
		return ErrInvalidInput
	}
	return s.repo.SetFeeBPS(ctx, merchantID, bps, s.clock.Now())
}

// PayoutFeeBPS returns the merchant's payout fee in hundredths of a percent.
func (s *Service) PayoutFeeBPS(ctx context.Context, merchantID string) (int, error) {
	return s.repo.PayoutFeeBPS(ctx, merchantID)
}

// SetPayoutFeeBPS sets the payout fee, from 0 to 10000. New merchants start
// at 0.
func (s *Service) SetPayoutFeeBPS(ctx context.Context, merchantID string, bps int) error {
	if bps < 0 || bps > maxFeeBPS {
		return ErrInvalidInput
	}
	return s.repo.SetPayoutFeeBPS(ctx, merchantID, bps, s.clock.Now())
}
