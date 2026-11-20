package merchants

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/zellapay/zellapay/internal/audit"
)

// Decision is a staff member's answer to a change request.
type Decision struct {
	RequestID string
	Approve   bool
	Origin
}

// Decide approves or rejects a change request. Approving applies the change
// in the same transaction. The requester can never decide their own request.
func (s *Service) Decide(ctx context.Context, d Decision) (ChangeRequest, error) {
	d.Reason = strings.TrimSpace(d.Reason)
	if d.RequestID == "" || d.Reason == "" {
		return ChangeRequest{}, ErrInvalidInput
	}
	if s.auditor == nil {
		return ChangeRequest{}, errNoAuditor
	}
	return s.repo.DecideChangeRequest(ctx, d.RequestID, func(r ChangeRequest) (Outcome, error) {
		return s.outcome(r, d)
	})
}

// outcome applies the rules to the locked request and prepares what to write.
func (s *Service) outcome(r ChangeRequest, d Decision) (Outcome, error) {
	now := s.clock.Now()
	if r.effective(now).Status != RequestPending {
		return Outcome{}, ErrRequestClosed
	}
	if r.RequestedBy == d.By.ID {
		return Outcome{}, ErrSelfApproval
	}
	status, action := RequestRejected, "merchant.change_rejected"
	if d.Approve {
		status, action = RequestApproved, appliedAction(r.Kind)
	}
	e, err := s.auditor.Prepare(audit.Entry{
		Actor: d.By, Action: action, Reason: d.Reason, Source: d.Source,
		Target: audit.Target{Kind: "merchant", ID: r.MerchantID},
		Change: decisionChange(r),
	})
	if err != nil {
		return Outcome{}, fmt.Errorf("prepare audit entry: %w", err)
	}
	return Outcome{
		Status: status, DecidedBy: d.By.ID, DecidedAt: now, DecisionReason: d.Reason,
		Effects: Effects{Audit: []audit.Pending{e}},
	}, nil
}

func appliedAction(k ChangeKind) string {
	if k == ChangeActivate {
		return "merchant.activated"
	}
	return "merchant.fees_changed"
}

func decisionChange(r ChangeRequest) map[string]string {
	change := map[string]string{
		"request": r.ID, "kind": string(r.Kind), "requested_by": r.RequestedBy,
	}
	if r.FeeBPS != nil {
		change["fee_bps"] = strconv.Itoa(*r.FeeBPS)
	}
	if r.PayoutFeeBPS != nil {
		change["payout_fee_bps"] = strconv.Itoa(*r.PayoutFeeBPS)
	}
	return change
}
