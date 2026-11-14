package merchants

import (
	"context"
	"fmt"
	"time"
)

// RotateKey issues a replacement with the same settings and lets the old
// key keep working for overlap, so clients can switch without downtime.
func (s *Service) RotateKey(
	ctx context.Context, ref KeyRef, overlap time.Duration,
) (IssuedKey, error) {
	if overlap <= 0 {
		return IssuedKey{}, ErrInvalidInput
	}
	old, err := s.findKey(ctx, ref)
	if err != nil {
		return IssuedKey{}, err
	}
	parts, err := newKeyParts(s.rand, old.Mode)
	if err != nil {
		return IssuedKey{}, err
	}
	next := s.buildKey(KeySpec{
		MerchantID:  old.MerchantID,
		Mode:        old.Mode,
		Scopes:      old.Scopes,
		IPAllowlist: old.IPAllowlist,
		ExpiresAt:   old.ExpiresAt,
	}, parts)
	expires := s.clock.Now().Add(overlap)
	if err := s.repo.RotateKey(ctx, ref, next, expires); err != nil {
		return IssuedKey{}, fmt.Errorf("rotate key: %w", err)
	}
	return IssuedKey{Key: next, Secret: parts.raw()}, nil
}

func (s *Service) findKey(ctx context.Context, ref KeyRef) (APIKey, error) {
	keys, err := s.repo.ListKeys(ctx, ref.MerchantID)
	if err != nil {
		return APIKey{}, err
	}
	for _, k := range keys {
		if k.ID == ref.KeyID && k.RevokedAt == nil {
			return k, nil
		}
	}
	return APIKey{}, ErrNotFound
}

// ListKeys returns a merchant's keys without their secrets.
func (s *Service) ListKeys(ctx context.Context, merchantID string) ([]APIKey, error) {
	return s.repo.ListKeys(ctx, merchantID)
}
