package merchants

import (
	"context"
	"errors"
	"testing"

	"github.com/zellapay/zellapay/internal/audit"
)

var staffer = audit.Actor{Kind: audit.ActorStaff, ID: "stf_1"}

func change(e env, reason string) StatusChange {
	return StatusChange{MerchantID: e.mer.ID, By: staffer, Reason: reason}
}

func TestSuspendAndRestoreAreAuditedWithTheReason(t *testing.T) {
	e := auditedEnv(t)
	ctx := context.Background()
	m, err := e.svc.Suspend(ctx, change(e, "chargeback spike"))
	if err != nil || m.Status != StatusSuspended {
		t.Fatalf("suspend = %v, %v", m.Status, err)
	}
	m, err = e.svc.Restore(ctx, change(e, "cleared after review"))
	if err != nil || m.Status != StatusActive {
		t.Fatalf("restore = %v, %v", m.Status, err)
	}
	got := e.repo.audits
	if len(got) != 2 || got[0].Action != "merchant.suspended" ||
		got[1].Action != "merchant.restored" || got[0].Reason != "chargeback spike" {
		t.Fatalf("audit entries = %+v", got)
	}
}

func TestStatusChangesAreGuardedByTheCurrentState(t *testing.T) {
	e := auditedEnv(t)
	ctx := context.Background()
	_, err := e.svc.Restore(ctx, change(e, "not suspended"))
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("restore active = %v", err)
	}
	if _, err := e.svc.Suspend(ctx, change(e, "first")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Suspend(ctx, change(e, "again")); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("suspend twice = %v", err)
	}
	if len(e.repo.audits) != 1 {
		t.Fatalf("a refused change was audited: %d entries", len(e.repo.audits))
	}
}

func TestStatusChangeNeedsAReasonAndAnAuditor(t *testing.T) {
	e := auditedEnv(t)
	ctx := context.Background()
	if _, err := e.svc.Suspend(ctx, change(e, "  ")); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank reason = %v", err)
	}
	if _, err := e.svc.Suspend(ctx, change(e, "abc")); err == nil {
		t.Fatal("a reason shorter than the audit minimum was accepted")
	}
	bare := newEnv(t, ModeTest)
	if _, err := bare.svc.Suspend(ctx, change(bare, "no auditor")); !errors.Is(err, errNoAuditor) {
		t.Fatalf("no auditor = %v", err)
	}
}

func TestStatusChangeOfAnUnknownMerchant(t *testing.T) {
	e := auditedEnv(t)
	c := change(e, "no such merchant")
	c.MerchantID = "mer_missing"
	if _, err := e.svc.Suspend(context.Background(), c); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}
