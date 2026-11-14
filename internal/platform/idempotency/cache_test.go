package idempotency

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

type fakeCache struct {
	entries map[string]CachedEntry
	getErr  error
	setErr  error
	sets    int
}

func newFakeCache() *fakeCache { return &fakeCache{entries: map[string]CachedEntry{}} }

func (c *fakeCache) Get(_ context.Context, r Request) (CachedEntry, bool, error) {
	e, ok := c.entries[r.Scope+"/"+r.Key]
	return e, ok, c.getErr
}

func (c *fakeCache) Set(_ context.Context, r Request, resp Response, _ time.Duration) error {
	c.sets++
	if c.setErr == nil {
		c.entries[r.Scope+"/"+r.Key] = CachedEntry{Fingerprint: r.Fingerprint, Response: resp}
	}
	return c.setErr
}

func cachedStore(inner Store, c Cache) *CachedStore {
	return NewCachedStore(inner, c, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

var cacheReq = Request{Scope: "m1", Key: "k1", Fingerprint: []byte("fp")}

func TestCachedStoreServesReplayWithoutTouchingTheStore(t *testing.T) {
	inner, cache := newMemStore(), newFakeCache()
	cs := cachedStore(inner, cache)
	ctx := context.Background()
	if _, err := cs.Begin(ctx, cacheReq, time.Time{}); err != nil {
		t.Fatal(err)
	}
	resp := Response{Status: 201, Body: []byte("x")}
	if err := cs.Complete(ctx, cacheReq, resp, time.Time{}); err != nil {
		t.Fatal(err)
	}
	inner.recs = map[string]*memRecord{} // the store forgets; the cache remembers
	d, err := cs.Begin(ctx, cacheReq, time.Time{})
	if err != nil || d.Kind != Replay || d.Response.Status != 201 {
		t.Fatalf("got %+v, %v", d, err)
	}
}

func TestCachedStoreDetectsConflictFromCache(t *testing.T) {
	cache := newFakeCache()
	cache.entries["m1/k1"] = CachedEntry{Fingerprint: []byte("other")}
	d, err := cachedStore(newMemStore(), cache).Begin(context.Background(), cacheReq, time.Time{})
	if err != nil || d.Kind != Conflict {
		t.Fatalf("got %+v, %v", d, err)
	}
}

func TestCachedStoreFallsBackWhenCacheFails(t *testing.T) {
	cache := newFakeCache()
	cache.getErr, cache.setErr = errors.New("redis down"), errors.New("redis down")
	cs := cachedStore(newMemStore(), cache)
	ctx := context.Background()
	d, err := cs.Begin(ctx, cacheReq, time.Time{})
	if err != nil || d.Kind != Started {
		t.Fatalf("begin: %+v, %v", d, err)
	}
	if err := cs.Complete(ctx, cacheReq, Response{Status: 200}, time.Time{}); err != nil {
		t.Fatalf("a cache write failure must not fail the request: %v", err)
	}
}

func TestCachedStoreDoesNotCacheWhenStoreFails(t *testing.T) {
	cache := newFakeCache()
	cs := cachedStore(newMemStore(), cache)
	// Completing a key that was never begun panics in memStore; use a
	// store that reports a lost lock instead.
	cs.inner = lostLockStore{}
	err := cs.Complete(context.Background(), cacheReq, Response{Status: 200}, time.Time{})
	if !errors.Is(err, ErrLockLost) || cache.sets != 0 {
		t.Fatalf("err=%v sets=%d", err, cache.sets)
	}
}

type lostLockStore struct{ Store }

func (lostLockStore) Complete(context.Context, Request, Response, time.Time) error {
	return ErrLockLost
}
