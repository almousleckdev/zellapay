package pgstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/merchants/pgstore/sqlcgen"
	"github.com/zellapay/zellapay/internal/platform/pgconv"
)

const uniqueViolation = "23505"

func optInt(n *int) pgtype.Int4 {
	if n == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: pgconv.Int32(*n), Valid: true}
}

func intPtr(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int32)
	return &n
}

func requestFromRow(r sqlcgen.MerchantChangeRequest) merchants.ChangeRequest {
	var by *string
	if r.DecidedBy.Valid {
		by = &r.DecidedBy.String
	}
	return merchants.ChangeRequest{
		ID: r.ID, MerchantID: r.MerchantID, Kind: merchants.ChangeKind(r.Kind),
		FeeBPS: intPtr(r.FeeBps), PayoutFeeBPS: intPtr(r.PayoutFeeBps), Reason: r.Reason,
		RequestedBy: r.RequestedBy, RequestedAt: r.RequestedAt.Time, ExpiresAt: r.ExpiresAt.Time,
		Status: merchants.RequestStatus(r.Status), DecidedBy: by, DecidedAt: timePtr(r.DecidedAt),
		DecisionReason: r.DecisionReason.String,
	}
}

// CreateChangeRequest stores a request and its audit entries.
func (s *Store) CreateChangeRequest(
	ctx context.Context, r merchants.ChangeRequest, fx merchants.Effects,
) (merchants.ChangeRequest, error) {
	var row sqlcgen.MerchantChangeRequest
	err := s.inTx(ctx, fx, func(q *sqlcgen.Queries) error {
		err := q.ExpireStaleChangeRequests(ctx, sqlcgen.ExpireStaleChangeRequestsParams{
			MerchantID: r.MerchantID, Kind: string(r.Kind), Now: ts(r.RequestedAt),
		})
		if err != nil {
			return err
		}
		row, err = q.InsertChangeRequest(ctx, sqlcgen.InsertChangeRequestParams{
			ID: r.ID, MerchantID: r.MerchantID, Kind: string(r.Kind),
			FeeBps: optInt(r.FeeBPS), PayoutFeeBps: optInt(r.PayoutFeeBPS), Reason: r.Reason,
			RequestedBy: r.RequestedBy, RequestedAt: ts(r.RequestedAt), ExpiresAt: ts(r.ExpiresAt),
		})
		return err
	})
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == uniqueViolation {
		return merchants.ChangeRequest{}, merchants.ErrRequestOpen
	}
	if err != nil {
		return merchants.ChangeRequest{}, fmt.Errorf("create change request: %w", err)
	}
	return requestFromRow(row), nil
}

// DecideChangeRequest locks a request, asks fn for the outcome and, when it is
// an approval, applies the change before recording the decision.
func (s *Store) DecideChangeRequest(
	ctx context.Context, id string, fn merchants.DecideFunc,
) (merchants.ChangeRequest, error) {
	var decided merchants.ChangeRequest
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		row, err := q.GetChangeRequestForUpdate(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return merchants.ErrNotFound
		}
		if err != nil {
			return err
		}
		req := requestFromRow(row)
		out, err := fn(req)
		if err != nil {
			return err
		}
		if out.Status == merchants.RequestApproved {
			if err := applyChange(ctx, q, req, out); err != nil {
				return err
			}
		}
		if err := recordDecision(ctx, q, id, out); err != nil {
			return err
		}
		decided = withOutcome(req, out)
		return s.appendAudit(ctx, tx, out.Effects)
	})
	return decided, err
}

func withOutcome(r merchants.ChangeRequest, o merchants.Outcome) merchants.ChangeRequest {
	r.Status, r.DecidedBy, r.DecidedAt = o.Status, &o.DecidedBy, &o.DecidedAt
	r.DecisionReason = o.DecisionReason
	return r
}

func recordDecision(ctx context.Context, q *sqlcgen.Queries, id string, o merchants.Outcome) error {
	return q.DecideChangeRequest(ctx, sqlcgen.DecideChangeRequestParams{
		ID: id, Status: string(o.Status),
		DecidedBy:      pgtype.Text{String: o.DecidedBy, Valid: true},
		DecidedAt:      ts(o.DecidedAt),
		DecisionReason: pgtype.Text{String: o.DecisionReason, Valid: true},
	})
}

// applyChange makes the change an approved request asked for.
func applyChange(
	ctx context.Context, q *sqlcgen.Queries, r merchants.ChangeRequest, o merchants.Outcome,
) error {
	if r.Kind == merchants.ChangeActivate {
		n, err := q.ChangeMerchantStatus(ctx, sqlcgen.ChangeMerchantStatusParams{
			ID: r.MerchantID, FromStatus: string(merchants.StatusPending),
			ToStatus: string(merchants.StatusActive), Now: ts(o.DecidedAt),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return merchants.ErrInvalidTransition
		}
		return nil
	}
	if r.FeeBPS == nil || r.PayoutFeeBPS == nil {
		return merchants.ErrInvalidInput
	}
	if _, err := q.SetMerchantFeeBPS(ctx, sqlcgen.SetMerchantFeeBPSParams{
		ID: r.MerchantID, FeeBps: pgconv.Int32(*r.FeeBPS), Now: ts(o.DecidedAt),
	}); err != nil {
		return err
	}
	_, err := q.SetMerchantPayoutFeeBPS(ctx, sqlcgen.SetMerchantPayoutFeeBPSParams{
		ID: r.MerchantID, FeeBps: pgconv.Int32(*r.PayoutFeeBPS), Now: ts(o.DecidedAt),
	})
	return err
}

// ListChangeRequests returns requests, newest first.
func (s *Store) ListChangeRequests(
	ctx context.Context, f merchants.RequestFilter,
) ([]merchants.ChangeRequest, error) {
	rows, err := sqlcgen.New(s.pool).ListChangeRequests(ctx, sqlcgen.ListChangeRequestsParams{
		Status: string(f.Status), MerchantID: f.MerchantID, RowLimit: pgconv.Int32(f.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list change requests: %w", err)
	}
	out := make([]merchants.ChangeRequest, len(rows))
	for i, r := range rows {
		out[i] = requestFromRow(sqlcgen.MerchantChangeRequest{
			ID: r.ID, MerchantID: r.MerchantID, Kind: r.Kind, FeeBps: r.FeeBps,
			PayoutFeeBps: r.PayoutFeeBps, Reason: r.Reason, RequestedBy: r.RequestedBy,
			RequestedAt: r.RequestedAt, ExpiresAt: r.ExpiresAt, Status: r.Status,
			DecidedBy: r.DecidedBy, DecidedAt: r.DecidedAt, DecisionReason: r.DecisionReason,
		})
		out[i].MerchantName = r.MerchantName
	}
	return out, nil
}

// CountPendingChangeRequests counts requests still waiting at now.
func (s *Store) CountPendingChangeRequests(ctx context.Context, now time.Time) (int, error) {
	n, err := sqlcgen.New(s.pool).CountPendingChangeRequests(ctx, ts(now))
	return int(n), err
}
