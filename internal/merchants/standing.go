package merchants

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zellapay/zellapay/internal/audit"
)

// ErrInvalidTransition means the merchant is not in a state the change
// applies to, such as suspending a merchant that is already suspended.
var ErrInvalidTransition = errors.New("merchants: invalid status change")

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusActive, StatusSuspended:
		return true
	}
	return false
}

// StatusChange is a staff decision to suspend or restore a merchant. The
// reason is required and is kept in the audit log.
type StatusChange struct {
	MerchantID string
	By         audit.Actor
	Reason     string
	Source     audit.Source
}

// Suspend stops an active merchant from taking payments or using the API.
func (s *Service) Suspend(ctx context.Context, c StatusChange) (Merchant, error) {
	return s.change(ctx, c, transition{StatusActive, StatusSuspended, "merchant.suspended"})
}

// Restore returns a suspended merchant to active.
func (s *Service) Restore(ctx context.Context, c StatusChange) (Merchant, error) {
	return s.change(ctx, c, transition{StatusSuspended, StatusActive, "merchant.restored"})
}

// transition is a status change and the audit action that records it.
type transition struct {
	from, to Status
	action   string
}

func (s *Service) change(
	ctx context.Context, c StatusChange, t transition,
) (Merchant, error) {
	if c.MerchantID == "" || len(strings.TrimSpace(c.Reason)) == 0 {
		return Merchant{}, ErrInvalidInput
	}
	if s.auditor == nil {
		return Merchant{}, errNoAuditor
	}
	e, err := s.auditor.Prepare(audit.Entry{
		Actor: c.By, Action: t.action, Reason: strings.TrimSpace(c.Reason),
		Target: audit.Target{Kind: "merchant", ID: c.MerchantID},
		Change: map[string]string{"from": string(t.from), "to": string(t.to)},
		Source: c.Source,
	})
	if err != nil {
		return Merchant{}, fmt.Errorf("prepare audit entry: %w", err)
	}
	err = s.repo.ChangeStatus(ctx, StatusUpdate{
		ID: c.MerchantID, From: t.from, To: t.to, At: s.clock.Now(),
	}, Effects{Audit: []audit.Pending{e}})
	if err != nil {
		return Merchant{}, err
	}
	return s.repo.GetMerchant(ctx, c.MerchantID)
}
