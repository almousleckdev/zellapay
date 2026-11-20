package merchants

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/platform/ids"
)

// FeeChange asks for new fees, in hundredths of a percent.
type FeeChange struct {
	MerchantID   string
	FeeBPS       int
	PayoutFeeBPS int
	Origin
}

// RequestActivation asks to let a pending merchant take live payments. A
// second staff member must approve it.
func (s *Service) RequestActivation(
	ctx context.Context, merchantID string, o Origin,
) (ChangeRequest, error) {
	m, err := s.repo.GetMerchant(ctx, merchantID)
	if err != nil {
		return ChangeRequest{}, err
	}
	if m.Status != StatusPending {
		return ChangeRequest{}, ErrInvalidTransition
	}
	r := s.newRequest(merchantID, ChangeActivate, o)
	return s.createRequest(ctx, r, o, requestAudit{action: "merchant.activation_requested"})
}

// RequestFeeChange asks to change a merchant's fees. A second staff member
// must approve it.
func (s *Service) RequestFeeChange(ctx context.Context, c FeeChange) (ChangeRequest, error) {
	if c.FeeBPS < 0 || c.FeeBPS > maxFeeBPS || c.PayoutFeeBPS < 0 || c.PayoutFeeBPS > maxFeeBPS {
		return ChangeRequest{}, ErrInvalidInput
	}
	if _, err := s.repo.GetMerchant(ctx, c.MerchantID); err != nil {
		return ChangeRequest{}, err
	}
	r := s.newRequest(c.MerchantID, ChangeFees, c.Origin)
	r.FeeBPS, r.PayoutFeeBPS = &c.FeeBPS, &c.PayoutFeeBPS
	change := map[string]string{
		"fee_bps": strconv.Itoa(c.FeeBPS), "payout_fee_bps": strconv.Itoa(c.PayoutFeeBPS),
	}
	return s.createRequest(ctx, r, c.Origin, requestAudit{"merchant.fees_requested", change})
}

func (s *Service) newRequest(merchantID string, kind ChangeKind, o Origin) ChangeRequest {
	now := s.clock.Now()
	return ChangeRequest{
		ID: ids.New("chg_"), MerchantID: merchantID, Kind: kind,
		Reason: strings.TrimSpace(o.Reason), RequestedBy: o.By.ID,
		RequestedAt: now, ExpiresAt: now.Add(requestTTL), Status: RequestPending,
	}
}

// requestAudit is the audit entry that records a request being made.
type requestAudit struct {
	action string
	change map[string]string
}

func (s *Service) createRequest(
	ctx context.Context, r ChangeRequest, o Origin, a requestAudit,
) (ChangeRequest, error) {
	change, action := a.change, a.action
	if r.Reason == "" {
		return ChangeRequest{}, ErrInvalidInput
	}
	if s.auditor == nil {
		return ChangeRequest{}, errNoAuditor
	}
	if change == nil {
		change = map[string]string{}
	}
	change["request"] = r.ID
	e, err := s.auditor.Prepare(audit.Entry{
		Actor: o.By, Action: action, Reason: r.Reason, Change: change, Source: o.Source,
		Target: audit.Target{Kind: "merchant", ID: r.MerchantID},
	})
	if err != nil {
		return ChangeRequest{}, fmt.Errorf("prepare audit entry: %w", err)
	}
	return s.repo.CreateChangeRequest(ctx, r, Effects{Audit: []audit.Pending{e}})
}
