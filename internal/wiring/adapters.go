package wiring

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/zellapay/zellapay/internal/collections"
	collectionsapi "github.com/zellapay/zellapay/internal/collections/httpapi"
	"github.com/zellapay/zellapay/internal/webhooks"
)

// collectionFeed lets the webhooks module read the collections outbox
// without depending on the collections module.
type collectionFeed struct{ svc *collections.Service }

var _ webhooks.Feed = collectionFeed{}

func (f collectionFeed) Claim(
	ctx context.Context, now time.Time, limit int,
	handle webhooks.ClaimHandler,
) (int, error) {
	return f.svc.ClaimOutbox(ctx, now, limit,
		func(ctx context.Context, entries []collections.OutboxEntry) error {
			out := make([]webhooks.OutboxEntry, len(entries))
			for i, e := range entries {
				out[i] = webhooks.OutboxEntry{
					ID: e.ID, ObjectID: e.CollectionID, Type: e.EventType, CreatedAt: e.CreatedAt,
				}
			}
			return handle(ctx, out)
		})
}

// collectionSource describes a collection exactly as the merchant API does,
// so webhook payloads and API responses share one shape.
type collectionSource struct{ svc *collections.Service }

var _ webhooks.PayloadSource = collectionSource{}

func (s collectionSource) Object(
	ctx context.Context, e webhooks.OutboxEntry,
) (string, json.RawMessage, error) {
	c, err := s.svc.Get(ctx, e.ObjectID)
	if errors.Is(err, collections.ErrNotFound) {
		return "", nil, webhooks.ErrNotFound
	}
	if err != nil {
		return "", nil, err
	}
	raw, err := collectionsapi.PublicJSON(c)
	return c.MerchantID, raw, err
}
