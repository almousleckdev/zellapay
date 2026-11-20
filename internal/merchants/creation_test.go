package merchants

import (
	"context"
	"errors"
	"testing"
)

func TestCreateByStaffIsPendingAndAudited(t *testing.T) {
	e := auditedEnv(t)
	m, err := e.svc.Create(context.Background(), NewMerchant{
		Name: "  Boutique Soleil ", Currency: "XAF", By: staffer,
		Reason: "signed contract 2026-114",
	})
	if err != nil || m.Status != StatusPending || m.Name != "Boutique Soleil" {
		t.Fatalf("create = %+v, %v", m, err)
	}
	got := e.repo.audits
	if len(got) != 1 || got[0].Action != "merchant.created" ||
		got[0].Target.ID != m.ID || got[0].Reason != "signed contract 2026-114" {
		t.Fatalf("audit entries = %+v", got)
	}
}

func TestCreateByStaffValidates(t *testing.T) {
	e := auditedEnv(t)
	ctx := context.Background()
	for _, n := range []NewMerchant{
		{Name: "", Currency: "XAF", By: staffer, Reason: "valid reason"},
		{Name: "Shop", Currency: "xaf", By: staffer, Reason: "valid reason"},
		{Name: "Shop", Currency: "XAF", By: staffer, Reason: "   "},
		{Name: string(make([]byte, 101)), Currency: "XAF", By: staffer, Reason: "valid reason"},
	} {
		if _, err := e.svc.Create(ctx, n); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%+v = %v", n, err)
		}
	}
	if _, err := e.svc.Create(ctx, NewMerchant{
		Name: "Shop", Currency: "XAF", By: staffer, Reason: "abc",
	}); err == nil {
		t.Fatal("a reason below the audit minimum was accepted")
	}
	if len(e.repo.audits) != 0 {
		t.Fatalf("refused creations were audited: %d", len(e.repo.audits))
	}
}

func TestCreateByStaffNeedsAnAuditor(t *testing.T) {
	e := newEnv(t, ModeTest)
	_, err := e.svc.Create(context.Background(), NewMerchant{
		Name: "Shop", Currency: "XAF", By: staffer, Reason: "valid reason",
	})
	if !errors.Is(err, errNoAuditor) {
		t.Fatalf("err = %v", err)
	}
}

func TestOverviewCountsByStatus(t *testing.T) {
	e := newEnv(t, ModeTest) // one active merchant "Acme"
	seedMerchants(t, e, 2)   // two pending
	o, err := e.svc.Overview(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if o.Total != 3 || o.ByStatus[StatusActive] != 1 || o.ByStatus[StatusPending] != 2 ||
		o.ByStatus[StatusSuspended] != 0 {
		t.Fatalf("counts = %+v", o)
	}
}

func TestOverviewFillsQuietDaysOldestFirst(t *testing.T) {
	e := newEnv(t, ModeTest)
	seedMerchants(t, e, 2) // three merchants, all created on the last day
	o, err := e.svc.Overview(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(o.CreatedBy) != 7 || o.CreatedBy[6].Count != 3 || o.CreatedBy[0].Count != 0 {
		t.Fatalf("per day = %+v", o.CreatedBy)
	}
	if !o.CreatedBy[1].Day.After(o.CreatedBy[0].Day) {
		t.Fatal("days are not oldest first")
	}
}

func TestOverviewRefusesBadRanges(t *testing.T) {
	e := newEnv(t, ModeTest)
	for _, days := range []int{0, -1, 91} {
		_, err := e.svc.Overview(context.Background(), days)
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("days=%d: %v", days, err)
		}
	}
}
