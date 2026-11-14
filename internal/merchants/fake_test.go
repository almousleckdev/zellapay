package merchants

import (
	"context"
	"sync"
	"time"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

// fakeRepo is an in-memory Repository for unit tests.
type fakeRepo struct {
	mu         sync.Mutex
	merchants  map[string]Merchant
	keys       map[string]APIKey
	fees       map[string]int
	payoutFees map[string]int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		merchants: map[string]Merchant{}, keys: map[string]APIKey{}, fees: map[string]int{},
		payoutFees: map[string]int{},
	}
}

func (r *fakeRepo) CreateMerchant(_ context.Context, m Merchant) (Merchant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.merchants[m.ID] = m
	return m, nil
}

func (r *fakeRepo) GetMerchant(_ context.Context, id string) (Merchant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.merchants[id]
	if !ok {
		return Merchant{}, ErrNotFound
	}
	return m, nil
}

func (r *fakeRepo) SetStatus(_ context.Context, id string, s Status, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.merchants[id]
	if !ok {
		return ErrNotFound
	}
	m.Status = s
	r.merchants[id] = m
	return nil
}

func (r *fakeRepo) FeeBPS(_ context.Context, id string) (int, error) {
	if _, ok := r.merchants[id]; !ok {
		return 0, ErrNotFound
	}
	return r.fees[id], nil
}

func (r *fakeRepo) SetFeeBPS(_ context.Context, id string, bps int, _ time.Time) error {
	if _, ok := r.merchants[id]; !ok {
		return ErrNotFound
	}
	r.fees[id] = bps
	return nil
}

func (r *fakeRepo) PayoutFeeBPS(_ context.Context, id string) (int, error) {
	if _, ok := r.merchants[id]; !ok {
		return 0, ErrNotFound
	}
	return r.payoutFees[id], nil
}

func (r *fakeRepo) SetPayoutFeeBPS(_ context.Context, id string, bps int, _ time.Time) error {
	if _, ok := r.merchants[id]; !ok {
		return ErrNotFound
	}
	r.payoutFees[id] = bps
	return nil
}

func (r *fakeRepo) InsertKey(_ context.Context, k APIKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys[k.ID] = k
	return nil
}

func (r *fakeRepo) KeyByLookup(_ context.Context, lookup string) (StoredKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, k := range r.keys {
		if k.LookupID == lookup {
			return StoredKey{Key: k, MerchantStatus: r.merchants[k.MerchantID].Status}, nil
		}
	}
	return StoredKey{}, ErrNotFound
}

func (r *fakeRepo) RevokeKey(_ context.Context, merchantID, keyID string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k, ok := r.keys[keyID]
	if !ok || k.MerchantID != merchantID || k.RevokedAt != nil {
		return ErrNotFound
	}
	k.RevokedAt = &now
	r.keys[keyID] = k
	return nil
}

func (r *fakeRepo) RotateKey(_ context.Context, old KeyRef, next APIKey, exp time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k, ok := r.keys[old.KeyID]
	if !ok || k.MerchantID != old.MerchantID {
		return ErrNotFound
	}
	k.ExpiresAt = &exp
	r.keys[old.KeyID] = k
	r.keys[next.ID] = next
	return nil
}

func (r *fakeRepo) ListKeys(_ context.Context, merchantID string) ([]APIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []APIKey
	for _, k := range r.keys {
		if k.MerchantID == merchantID {
			out = append(out, k)
		}
	}
	return out, nil
}
