package idempotency

import (
	"bytes"
	"context"
	"log/slog"
	"time"
)

// CachedEntry is a completed request held in a fast cache.
type CachedEntry struct {
	Fingerprint []byte
	Response    Response
}

// Cache is a best-effort lookaside store for completed requests. It may
// lose entries at any time; PostgreSQL remains the source of truth.
type Cache interface {
	// Get looks up the request by Scope and Key.
	Get(ctx context.Context, req Request) (CachedEntry, bool, error)
	// Set stores resp together with the request's fingerprint.
	Set(ctx context.Context, req Request, resp Response, ttl time.Duration) error
}

// CachedStore answers replays from a Cache and falls back to inner. Cache
// failures are logged and never fail a request.
type CachedStore struct {
	inner Store
	cache Cache
	ttl   time.Duration
	log   *slog.Logger
}

var _ Store = (*CachedStore)(nil)

// NewCachedStore wraps inner. ttl must not exceed the record lifetime of
// inner, or the cache could replay a record the store has already expired.
func NewCachedStore(inner Store, cache Cache, ttl time.Duration, log *slog.Logger) *CachedStore {
	return &CachedStore{inner: inner, cache: cache, ttl: ttl, log: log}
}

// Begin serves a replay or conflict from the cache when it can.
func (c *CachedStore) Begin(ctx context.Context, req Request, now time.Time) (Decision, error) {
	entry, found, err := c.cache.Get(ctx, req)
	if err != nil {
		c.log.Warn("idempotency cache read failed", "err", err)
	}
	if err == nil && found {
		if bytes.Equal(entry.Fingerprint, req.Fingerprint) {
			return Decision{Kind: Replay, Response: entry.Response}, nil
		}
		return Decision{Kind: Conflict}, nil
	}
	return c.inner.Begin(ctx, req, now)
}

// Complete stores the response durably, then caches it.
func (c *CachedStore) Complete(
	ctx context.Context, req Request, resp Response, now time.Time,
) error {
	if err := c.inner.Complete(ctx, req, resp, now); err != nil {
		return err
	}
	if err := c.cache.Set(ctx, req, resp, c.ttl); err != nil {
		c.log.Warn("idempotency cache write failed", "err", err)
	}
	return nil
}

// Release delegates to the store; in-progress claims are never cached.
func (c *CachedStore) Release(ctx context.Context, req Request) error {
	return c.inner.Release(ctx, req)
}
