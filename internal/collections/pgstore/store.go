// Package pgstore persists collections in PostgreSQL. Customer phone numbers
// are encrypted before they are written and looked up by a keyed index.
package pgstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/collections/pgstore/sqlcgen"
	"github.com/zellapay/zellapay/internal/platform/crypto"
)

const phoneIndexLabel = "collection.customer_phone"

// Store implements collections.Repository on PostgreSQL.
type Store struct {
	pool    *pgxpool.Pool
	sealer  *crypto.Sealer
	indexer *crypto.BlindIndexer
}

var _ collections.Repository = (*Store)(nil)

// New returns a Store that protects phone numbers with sealer and indexer.
func New(pool *pgxpool.Pool, sealer *crypto.Sealer, indexer *crypto.BlindIndexer) *Store {
	return &Store{pool: pool, sealer: sealer, indexer: indexer}
}

// phoneContext binds a sealed phone number to its collection.
func phoneContext(collectionID string) []byte {
	return []byte("collection:" + collectionID + ":customer_phone")
}

// Create inserts nc, or returns the earlier collection that used the same
// idempotency key when the request is identical.
func (s *Store) Create(
	ctx context.Context, nc collections.NewCollection,
) (collections.Collection, bool, error) {
	sealed, err := s.sealer.Seal([]byte(nc.Phone), phoneContext(nc.ID))
	if err != nil {
		return collections.Collection{}, false, fmt.Errorf("seal phone: %w", err)
	}
	nc.PhoneIndex = s.indexer.Index(phoneIndexLabel, nc.Phone)
	q := sqlcgen.New(s.pool)
	_, err = q.InsertCollection(ctx, insertParams(nc, sealed))
	if errors.Is(err, pgx.ErrNoRows) {
		return s.existing(ctx, q, nc)
	}
	if err != nil {
		return collections.Collection{}, false, err
	}
	c, err := s.Get(ctx, nc.ID)
	return c, true, err
}

// existing resolves an idempotency-key collision: the same request returns
// the original, a different one is a conflict.
func (s *Store) existing(
	ctx context.Context, q *sqlcgen.Queries, nc collections.NewCollection,
) (collections.Collection, bool, error) {
	row, err := q.GetCollectionByKey(ctx, sqlcgen.GetCollectionByKeyParams{
		MerchantID: nc.MerchantID, IdempotencyKey: nc.IdempotencyKey,
	})
	if err != nil {
		return collections.Collection{}, false, err
	}
	c := fromRow(sqlcgen.GetCollectionRow(row))
	same := c.Amount == nc.Amount && c.Currency == nc.Currency &&
		c.MerchantReference == nc.MerchantReference &&
		bytes.Equal(c.PhoneIndex, nc.PhoneIndex)
	if !same {
		return collections.Collection{}, false, collections.ErrIdempotencyConflict
	}
	return c, false, nil
}

// Get loads a collection by ID.
func (s *Store) Get(ctx context.Context, id string) (collections.Collection, error) {
	row, err := sqlcgen.New(s.pool).GetCollection(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return collections.Collection{}, collections.ErrNotFound
	}
	if err != nil {
		return collections.Collection{}, err
	}
	return fromRow(row), nil
}
