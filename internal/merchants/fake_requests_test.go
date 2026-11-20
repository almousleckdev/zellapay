package merchants

import (
	"context"
	"time"
)

func (r *fakeRepo) CreateChangeRequest(
	_ context.Context, c ChangeRequest, fx Effects,
) (ChangeRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, old := range r.requests {
		if old.MerchantID != c.MerchantID || old.Kind != c.Kind || old.Status != RequestPending {
			continue
		}
		if old.ExpiresAt.After(c.RequestedAt) {
			return ChangeRequest{}, ErrRequestOpen
		}
		r.requests[i].Status = RequestExpired
	}
	r.requests = append(r.requests, c)
	r.audits = append(r.audits, fx.Audit...)
	return c, nil
}

func (r *fakeRepo) DecideChangeRequest(
	_ context.Context, id string, fn DecideFunc,
) (ChangeRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, req := range r.requests {
		if req.ID != id {
			continue
		}
		out, err := fn(req)
		if err != nil {
			return ChangeRequest{}, err
		}
		if out.Status == RequestApproved {
			r.apply(req)
		}
		by, at := out.DecidedBy, out.DecidedAt
		cur := &r.requests[i]
		cur.Status, cur.DecidedBy, cur.DecidedAt = out.Status, &by, &at
		cur.DecisionReason = out.DecisionReason
		r.audits = append(r.audits, out.Effects.Audit...)
		return r.requests[i], nil
	}
	return ChangeRequest{}, ErrNotFound
}

func (r *fakeRepo) apply(req ChangeRequest) {
	if req.Kind == ChangeActivate {
		m := r.merchants[req.MerchantID]
		m.Status = StatusActive
		r.merchants[req.MerchantID] = m
		return
	}
	r.fees[req.MerchantID] = *req.FeeBPS
	r.payoutFees[req.MerchantID] = *req.PayoutFeeBPS
}

func (r *fakeRepo) ListChangeRequests(_ context.Context, f RequestFilter) ([]ChangeRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []ChangeRequest
	for _, req := range r.requests {
		if (f.Status == "" || req.Status == f.Status) &&
			(f.MerchantID == "" || req.MerchantID == f.MerchantID) {
			out = append(out, req)
		}
	}
	return out, nil
}

func (r *fakeRepo) CountPendingChangeRequests(_ context.Context, now time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, req := range r.requests {
		if req.Status == RequestPending && req.ExpiresAt.After(now) {
			n++
		}
	}
	return n, nil
}
