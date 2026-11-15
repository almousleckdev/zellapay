// Package pgstore keeps the audit log in PostgreSQL. The database enforces the
// chain itself: it refuses edits, deletions, truncation and any entry that
// does not extend the current head.
package pgstore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/audit/pgstore/sqlcgen"
	"github.com/zellapay/zellapay/internal/platform/pgconv"
)

const maxBatch = 5000

var ts = pgconv.Timestamptz

// Store implements audit.Repository on PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

var _ audit.Repository = (*Store)(nil)

// New returns a Store backed by pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Append chains p after the current head in its own transaction.
func (s *Store) Append(ctx context.Context, p audit.Pending) (audit.Record, error) {
	var rec audit.Record
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		rec, err = s.AppendIn(ctx, tx, p)
		return err
	})
	return rec, err
}

// AppendIn chains p inside tx, so the entry exists exactly when the action it
// describes does. It locks the head row until the transaction ends, so call
// it last, after the transaction's other work, to keep that lock short.
func (s *Store) AppendIn(ctx context.Context, tx pgx.Tx, p audit.Pending) (audit.Record, error) {
	q := sqlcgen.New(tx)
	head, err := q.LockAuditHead(ctx)
	if err != nil {
		return audit.Record{}, fmt.Errorf("lock head: %w", err)
	}
	prev, err := toHash(head.Hash)
	if err != nil {
		return audit.Record{}, err
	}
	rec := audit.Record{
		Entry: p.Entry, Seq: head.Seq + 1, At: p.At, ChangeJSON: p.ChangeJSON, PrevHash: prev,
	}
	rec.Hash = audit.Link(prev, rec.Seq, p)
	if err := q.InsertAuditEntry(ctx, insertParams(rec)); err != nil {
		return audit.Record{}, fmt.Errorf("insert entry: %w", err)
	}
	err = q.AdvanceAuditHead(ctx, sqlcgen.AdvanceAuditHeadParams{Seq: rec.Seq, Hash: rec.Hash[:]})
	return rec, err
}

// Head returns the newest entry's position and hash without locking.
func (s *Store) Head(ctx context.Context) (audit.Head, error) {
	h, err := sqlcgen.New(s.pool).GetAuditHead(ctx)
	if err != nil {
		return audit.Head{}, err
	}
	hash, err := toHash(h.Hash)
	return audit.Head{Seq: h.Seq, Hash: hash}, err
}

func insertParams(r audit.Record) sqlcgen.InsertAuditEntryParams {
	return sqlcgen.InsertAuditEntryParams{
		Seq: r.Seq, At: ts(r.At), ActorKind: string(r.Actor.Kind), ActorID: r.Actor.ID,
		Action: r.Action, TargetKind: r.Target.Kind, TargetID: r.Target.ID,
		Reason: r.Reason, Change: r.ChangeJSON, Ip: r.Source.IP,
		UserAgent: r.Source.UserAgent, RequestID: r.Source.RequestID,
		PrevHash: r.PrevHash[:], Hash: r.Hash[:],
	}
}

func toHash(b []byte) (audit.Hash, error) {
	var h audit.Hash
	if len(b) != len(h) {
		return h, fmt.Errorf("audit: stored hash has %d bytes", len(b))
	}
	copy(h[:], b)
	return h, nil
}

func recordFrom(r sqlcgen.AuditLog) (audit.Record, error) {
	prev, err := toHash(r.PrevHash)
	if err != nil {
		return audit.Record{}, err
	}
	hash, err := toHash(r.Hash)
	if err != nil {
		return audit.Record{}, err
	}
	change := map[string]string{}
	if err := json.Unmarshal([]byte(r.Change), &change); err != nil {
		return audit.Record{}, fmt.Errorf("audit: stored change is not valid: %w", err)
	}
	return audit.Record{
		Entry: audit.Entry{
			Actor:  audit.Actor{Kind: audit.ActorKind(r.ActorKind), ID: r.ActorID},
			Action: r.Action, Target: audit.Target{Kind: r.TargetKind, ID: r.TargetID},
			Reason: r.Reason, Change: change,
			Source: audit.Source{IP: r.Ip, UserAgent: r.UserAgent, RequestID: r.RequestID},
		},
		Seq: r.Seq, At: r.At.Time.UTC(), ChangeJSON: r.Change, PrevHash: prev, Hash: hash,
	}, nil
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
