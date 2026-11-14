// Package pgstore persists webhook endpoints, events and deliveries.
package pgstore

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zellapay/zellapay/internal/platform/pgconv"
	"github.com/zellapay/zellapay/internal/webhooks"
	"github.com/zellapay/zellapay/internal/webhooks/pgstore/sqlcgen"
)

var (
	ts      = pgconv.Timestamptz
	optTS   = pgconv.OptTimestamptz
	timePtr = pgconv.TimePtr
)

// Store implements webhooks.Repository on PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

var _ webhooks.Repository = (*Store)(nil)

// New returns a Store backed by pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// CreateEndpoint stores e, refusing the request if the merchant already has
// max active endpoints. The merchant row is locked so concurrent requests
// cannot both slip past the limit.
func (s *Store) CreateEndpoint(ctx context.Context, e webhooks.SealedEndpoint, max int) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		if _, err := q.LockMerchantForEndpoints(ctx, e.MerchantID); err != nil {
			return err
		}
		n, err := q.CountActiveEndpoints(ctx, e.MerchantID)
		if err != nil {
			return err
		}
		if int(n) >= max {
			return webhooks.ErrTooManyEndpoints
		}
		return q.InsertEndpoint(ctx, sqlcgen.InsertEndpointParams{
			ID: e.ID, MerchantID: e.MerchantID, Url: e.URL, SecretEnc: e.SecretSealed,
			Events: e.Events, Description: e.Description, CreatedAt: ts(e.CreatedAt),
		})
	})
}

// ListEndpoints returns all of a merchant's endpoints, without secrets.
func (s *Store) ListEndpoints(ctx context.Context, merchantID string) ([]webhooks.Endpoint, error) {
	rows, err := sqlcgen.New(s.pool).ListEndpoints(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	out := make([]webhooks.Endpoint, len(rows))
	for i, r := range rows {
		out[i] = endpointFrom(r)
	}
	return out, nil
}

// EndpointsFor returns active endpoints subscribed to eventType.
func (s *Store) EndpointsFor(
	ctx context.Context, merchantID, eventType string,
) ([]webhooks.Endpoint, error) {
	rows, err := sqlcgen.New(s.pool).EndpointsForEvent(ctx, sqlcgen.EndpointsForEventParams{
		MerchantID: merchantID, EventType: eventType,
	})
	if err != nil {
		return nil, err
	}
	out := make([]webhooks.Endpoint, len(rows))
	for i, r := range rows {
		out[i] = endpointFrom(sqlcgen.ListEndpointsRow(r))
	}
	return out, nil
}

// endpointFrom converts a row. The endpoint queries select the same columns.
func endpointFrom(r sqlcgen.ListEndpointsRow) webhooks.Endpoint {
	return webhooks.Endpoint{
		ID: r.ID, MerchantID: r.MerchantID, URL: r.Url, Events: r.Events,
		Description: r.Description, Active: r.Status == "active", CreatedAt: r.CreatedAt.Time,
	}
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return webhooks.ErrNotFound
	}
	return err
}
