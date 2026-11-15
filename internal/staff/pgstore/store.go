// Package pgstore persists staff data in PostgreSQL.
package pgstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/pgconv"
	"github.com/zellapay/zellapay/internal/staff"
	"github.com/zellapay/zellapay/internal/staff/pgstore/sqlcgen"
)

const uniqueViolation = "23505"

var (
	ts      = pgconv.Timestamptz
	timePtr = pgconv.TimePtr
)

// AuditLog appends an entry to the audit log inside a transaction.
type AuditLog interface {
	AppendIn(ctx context.Context, tx pgx.Tx, p audit.Pending) (audit.Record, error)
}

// Store implements staff.Repository on PostgreSQL.
type Store struct {
	pool  *pgxpool.Pool
	audit AuditLog
}

var _ staff.Repository = (*Store)(nil)

// New returns a Store. auditLog is the audit module's store, written in the
// same transaction as the change it records.
func New(pool *pgxpool.Pool, auditLog AuditLog) *Store {
	return &Store{pool: pool, audit: auditLog}
}

// inTx runs work, then appends the audit entries, in one transaction.
func (s *Store) inTx(
	ctx context.Context, fx staff.Effects, work func(*sqlcgen.Queries) error,
) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := work(sqlcgen.New(tx)); err != nil {
			return err
		}
		for _, e := range fx.Audit {
			if _, err := s.audit.AppendIn(ctx, tx, e); err != nil {
				return fmt.Errorf("append audit entry: %w", err)
			}
		}
		return nil
	})
}

func isUnique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == uniqueViolation
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.ErrNotFound
	}
	return err
}
