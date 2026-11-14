// Package rediscache implements idempotency.Cache on Redis.
package rediscache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/zellapay/zellapay/internal/platform/idempotency"
)

// Cache stores completed idempotent responses in Redis.
type Cache struct {
	client redis.Cmdable
}

var _ idempotency.Cache = (*Cache)(nil)

// New returns a Cache using client.
func New(client redis.Cmdable) *Cache { return &Cache{client: client} }

type wire struct {
	Fingerprint []byte `json:"fp"`
	Status      int    `json:"status"`
	ContentType string `json:"ct"`
	Body        []byte `json:"body"`
}

func redisKey(scope, key string) string {
	return "idem:" + scope + ":" + key
}

// Get returns the cached entry, if any.
func (c *Cache) Get(
	ctx context.Context, req idempotency.Request,
) (idempotency.CachedEntry, bool, error) {
	raw, err := c.client.Get(ctx, redisKey(req.Scope, req.Key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return idempotency.CachedEntry{}, false, nil
	}
	if err != nil {
		return idempotency.CachedEntry{}, false, err
	}
	var w wire
	if err := json.Unmarshal(raw, &w); err != nil {
		return idempotency.CachedEntry{}, false, err
	}
	return idempotency.CachedEntry{
		Fingerprint: w.Fingerprint,
		Response: idempotency.Response{
			Status: w.Status, ContentType: w.ContentType, Body: w.Body,
		},
	}, true, nil
}

// Set stores resp and the request fingerprint for ttl.
func (c *Cache) Set(
	ctx context.Context, req idempotency.Request, resp idempotency.Response, ttl time.Duration,
) error {
	raw, err := json.Marshal(wire{
		Fingerprint: req.Fingerprint,
		Status:      resp.Status,
		ContentType: resp.ContentType,
		Body:        resp.Body,
	})
	if err != nil {
		return err
	}
	return c.client.Set(ctx, redisKey(req.Scope, req.Key), raw, ttl).Err()
}
