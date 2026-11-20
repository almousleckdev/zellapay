package merchants

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zellapay/zellapay/internal/audit"
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
	audits     []audit.Pending
	requests   []ChangeRequest
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		merchants: map[string]Merchant{}, keys: map[string]APIKey{}, fees: map[string]int{},
		payoutFees: map[string]int{},
	}
}

func (r *fakeRepo) CreateMerchant(_ context.Context, m Merchant, fx Effects) (Merchant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.merchants[m.ID] = m
	r.audits = append(r.audits, fx.Audit...)
	return m, nil
}

func (r *fakeRepo) MerchantCounts(_ context.Context) (map[Status]int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[Status]int{}
	for _, m := range r.merchants {
		out[m.Status]++
	}
	return out, nil
}

func (r *fakeRepo) CreatedPerDay(_ context.Context, since time.Time) ([]DayCount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	per := map[time.Time]int{}
	for _, m := range r.merchants {
		if m.CreatedAt.Before(since) {
			continue
		}
		d := m.CreatedAt.In(gabon)
		per[time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, gabon)]++
	}
	var out []DayCount
	for d, n := range per {
		out = append(out, DayCount{Day: d, Count: n})
	}
	return out, nil
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

func (r *fakeRepo) InsertKey(_ context.Context, k APIKey, fx Effects) error {
	r.audits = append(r.audits, fx.Audit...)
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

func (r *fakeRepo) RevokeKey(_ context.Context, ref KeyRef, now time.Time, fx Effects) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	keyID := ref.KeyID
	k, ok := r.keys[keyID]
	if !ok || k.MerchantID != ref.MerchantID || k.RevokedAt != nil {
		return ErrNotFound
	}
	k.RevokedAt = &now
	r.keys[keyID] = k
	r.audits = append(r.audits, fx.Audit...)
	return nil
}

func (r *fakeRepo) RotateKey(_ context.Context, rot Rotation, fx Effects) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, next, exp := rot.Old, rot.Next, rot.OldExpires
	r.audits = append(r.audits, fx.Audit...)
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

func (r *fakeRepo) ListMerchants(_ context.Context, q ListQuery) ([]Merchant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var all []Merchant
	for _, m := range r.merchants {
		if q.Status != "" && m.Status != q.Status {
			continue
		}
		if q.Search != "" && !strings.Contains(strings.ToLower(m.Name), strings.ToLower(q.Search)) {
			continue
		}
		if q.After != nil && !olderThan(m, *q.After) {
			continue
		}
		all = append(all, m)
	}
	sort.Slice(all, func(i, j int) bool { return newer(all[i], all[j]) })
	if len(all) > q.Limit {
		all = all[:q.Limit]
	}
	return all, nil
}

func newer(a, b Merchant) bool {
	if a.CreatedAt.Equal(b.CreatedAt) {
		return a.ID > b.ID
	}
	return a.CreatedAt.After(b.CreatedAt)
}

func olderThan(m Merchant, c Cursor) bool {
	return newer(Merchant{ID: c.ID, CreatedAt: c.CreatedAt}, m)
}

func (r *fakeRepo) ChangeStatus(_ context.Context, u StatusUpdate, fx Effects) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.merchants[u.ID]
	if !ok {
		return ErrNotFound
	}
	if m.Status != u.From {
		return ErrInvalidTransition
	}
	m.Status = u.To
	r.merchants[u.ID] = m
	r.audits = append(r.audits, fx.Audit...)
	return nil
}
