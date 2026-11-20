package merchants

import (
	"context"
	"fmt"
	"strings"

	"github.com/zellapay/zellapay/internal/audit"
)

// NewMerchant is a staff decision to register a merchant. The reason is
// required and is kept in the audit log.
type NewMerchant struct {
	Name     string
	Currency string
	By       audit.Actor
	Reason   string
	Source   audit.Source
}

// Create registers a merchant in the pending state, audited with the reason.
func (s *Service) Create(ctx context.Context, n NewMerchant) (Merchant, error) {
	if len(strings.TrimSpace(n.Reason)) == 0 {
		return Merchant{}, ErrInvalidInput
	}
	if s.auditor == nil {
		return Merchant{}, errNoAuditor
	}
	m, err := s.newMerchant(n.Name, n.Currency)
	if err != nil {
		return Merchant{}, err
	}
	e, err := s.auditor.Prepare(audit.Entry{
		Actor: n.By, Action: "merchant.created", Reason: strings.TrimSpace(n.Reason),
		Target: audit.Target{Kind: "merchant", ID: m.ID},
		Change: map[string]string{"currency": m.DefaultCurrency},
		Source: n.Source,
	})
	if err != nil {
		return Merchant{}, fmt.Errorf("prepare audit entry: %w", err)
	}
	return s.repo.CreateMerchant(ctx, m, Effects{Audit: []audit.Pending{e}})
}
