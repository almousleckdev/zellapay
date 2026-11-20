package pgstore_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/audit"
	auditstore "github.com/zellapay/zellapay/internal/audit/pgstore"
	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/merchants/pgstore"
	"github.com/zellapay/zellapay/internal/platform/clock"
	"github.com/zellapay/zellapay/internal/platform/pgtest"
)

func newService(t *testing.T) (*merchants.Service, merchants.Merchant) {
	t.Helper()
	repo := pgstore.New(pgtest.NewPool(t, testDSN))
	svc := merchants.NewService(repo, clock.System{}, merchants.ModeTest)
	m, err := svc.CreateMerchant(context.Background(), "Acme", "XAF")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetStatus(context.Background(), m.ID, merchants.StatusActive); err != nil {
		t.Fatal(err)
	}
	return svc, m
}

func issue(t *testing.T, svc *merchants.Service, m merchants.Merchant) merchants.IssuedKey {
	t.Helper()
	k, err := svc.IssueKey(context.Background(), merchants.KeySpec{
		MerchantID:  m.ID,
		Mode:        merchants.ModeTest,
		Scopes:      []merchants.Scope{merchants.ScopeRead, merchants.ScopeCollectionsWrite},
		IPAllowlist: []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")},
	})
	if err != nil {
		t.Fatal(err)
	}
	return k
}

var okIP = netip.MustParseAddr("203.0.113.9")

func TestIssueAndAuthenticate(t *testing.T) {
	svc, m := newService(t)
	k := issue(t, svc, m)

	p, err := svc.Authenticate(context.Background(), k.Secret, okIP)
	if err != nil {
		t.Fatal(err)
	}
	if p.MerchantID != m.ID || !p.Has(merchants.ScopeCollectionsWrite) {
		t.Fatalf("unexpected principal %+v", p)
	}
	_, err = svc.Authenticate(context.Background(), k.Secret, netip.MustParseAddr("192.0.2.1"))
	if !errors.Is(err, merchants.ErrIPNotAllowed) {
		t.Fatalf("got %v", err)
	}
}

func TestRevokedKeyIsRejected(t *testing.T) {
	svc, m := newService(t)
	k := issue(t, svc, m)
	if err := svc.RevokeKey(context.Background(), m.ID, k.Key.ID); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Authenticate(context.Background(), k.Secret, okIP)
	if !errors.Is(err, merchants.ErrInvalidKey) {
		t.Fatalf("got %v", err)
	}
	err = svc.RevokeKey(context.Background(), m.ID, k.Key.ID)
	if !errors.Is(err, merchants.ErrNotFound) {
		t.Fatalf("second revoke: got %v", err)
	}
}

func TestRevokeRequiresOwningMerchant(t *testing.T) {
	svc, m := newService(t)
	k := issue(t, svc, m)
	err := svc.RevokeKey(context.Background(), "mer_someoneelse", k.Key.ID)
	if !errors.Is(err, merchants.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
	if _, err := svc.Authenticate(context.Background(), k.Secret, okIP); err != nil {
		t.Fatalf("key must still work: %v", err)
	}
}

func TestRotateKeepsBothKeysDuringOverlap(t *testing.T) {
	svc, m := newService(t)
	old := issue(t, svc, m)
	ref := merchants.KeyRef{MerchantID: m.ID, KeyID: old.Key.ID}
	next, err := svc.RotateKey(context.Background(), ref, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []merchants.IssuedKey{old, next} {
		if _, err := svc.Authenticate(context.Background(), k.Secret, okIP); err != nil {
			t.Fatalf("key must work during overlap: %v", err)
		}
	}
}

func TestSuspendedMerchantCannotAuthenticate(t *testing.T) {
	svc, m := newService(t)
	k := issue(t, svc, m)
	if err := svc.SetStatus(context.Background(), m.ID, merchants.StatusSuspended); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Authenticate(context.Background(), k.Secret, okIP)
	if !errors.Is(err, merchants.ErrMerchantInactive) {
		t.Fatalf("got %v", err)
	}
}

func TestSetStatusUnknownMerchant(t *testing.T) {
	svc, _ := newService(t)
	err := svc.SetStatus(context.Background(), "mer_missing", merchants.StatusActive)
	if !errors.Is(err, merchants.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestKeysCannotBeDeleted(t *testing.T) {
	svc, m := newService(t)
	k := issue(t, svc, m)
	pool := pgtest.NewPool(t, testDSN)
	_, err := pool.Exec(context.Background(),
		"DELETE FROM merchant_api_keys WHERE id = $1", k.Key.ID)
	if err == nil {
		t.Fatal("deleting an api key must be rejected by the database")
	}
}

func TestListSearchTreatsWildcardsAsPlainText(t *testing.T) {
	svc, _ := newService(t) // creates "Acme" or similar
	page, err := svc.List(context.Background(), merchants.ListQuery{Search: "%"})
	if err != nil || len(page.Merchants) != 0 {
		t.Fatalf("%% matched %d merchants, %v", len(page.Merchants), err)
	}
}

func TestChangeStatusIsGuardedAndAudited(t *testing.T) {
	pool := pgtest.NewPool(t, testDSN)
	log := auditstore.New(pool)
	svc := merchants.NewService(
		pgstore.New(pool).WithAudit(log), clock.System{}, merchants.ModeTest,
	).WithAuditor(audit.NewService(log, clock.System{}))
	ctx := context.Background()
	m, err := svc.CreateMerchant(ctx, "Acme", "XAF")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetStatus(ctx, m.ID, merchants.StatusActive); err != nil {
		t.Fatal(err)
	}
	c := merchants.StatusChange{
		MerchantID: m.ID, By: audit.Actor{Kind: audit.ActorStaff, ID: "stf_1"},
		Reason: "chargeback spike",
	}
	if _, err := svc.Restore(ctx, c); !errors.Is(err, merchants.ErrInvalidTransition) {
		t.Fatalf("restore active = %v", err)
	}
	got, err := svc.Suspend(ctx, c)
	if err != nil || got.Status != merchants.StatusSuspended {
		t.Fatalf("suspend = %+v, %v", got, err)
	}
	recs, err := audit.NewService(log, clock.System{}).List(ctx, audit.Filter{TargetID: m.ID})
	if err != nil || len(recs) != 1 || recs[0].Reason != "chargeback spike" {
		t.Fatalf("audit = %+v, %v", recs, err)
	}
	c.MerchantID = "mer_missing"
	if _, err := svc.Suspend(ctx, c); !errors.Is(err, merchants.ErrNotFound) {
		t.Fatalf("missing = %v", err)
	}
}

func auditedService(t *testing.T) *merchants.Service {
	t.Helper()
	pool := pgtest.NewPool(t, testDSN)
	log := auditstore.New(pool)
	return merchants.NewService(
		pgstore.New(pool).WithAudit(log), clock.System{}, merchants.ModeTest,
	).WithAuditor(audit.NewService(log, clock.System{}))
}

func staffer(id string) merchants.Origin {
	by := audit.Actor{Kind: audit.ActorStaff, ID: id}
	return merchants.Origin{By: by, Reason: "contract checked"}
}

func TestActivationNeedsTwoPeopleAndOnlyOneOpenRequest(t *testing.T) {
	svc := auditedService(t)
	ctx := context.Background()
	m, err := svc.CreateMerchant(ctx, "Soleil", "XAF")
	if err != nil {
		t.Fatal(err)
	}
	req, err := svc.RequestActivation(ctx, m.ID, staffer("stf_a"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.RequestActivation(ctx, m.ID, staffer("stf_b"))
	if !errors.Is(err, merchants.ErrRequestOpen) {
		t.Fatalf("second open request = %v", err)
	}
	self := merchants.Decision{RequestID: req.ID, Approve: true, Origin: staffer("stf_a")}
	if _, err := svc.Decide(ctx, self); !errors.Is(err, merchants.ErrSelfApproval) {
		t.Fatalf("self approval = %v", err)
	}
	other := merchants.Decision{RequestID: req.ID, Approve: true, Origin: staffer("stf_b")}
	if _, err := svc.Decide(ctx, other); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.GetMerchant(ctx, m.ID); got.Status != merchants.StatusActive {
		t.Fatalf("status = %s", got.Status)
	}
	pending, _ := svc.ListRequests(ctx, merchants.RequestFilter{Status: merchants.RequestPending})
	for _, r := range pending {
		if r.ID == req.ID {
			t.Fatal("a decided request is still listed as pending")
		}
	}
}

func TestFeeChangeAppliesBothFeesOnApproval(t *testing.T) {
	svc := auditedService(t)
	ctx := context.Background()
	m, _ := svc.CreateMerchant(ctx, "Soleil", "XAF")
	req, err := svc.RequestFeeChange(ctx, merchants.FeeChange{
		MerchantID: m.ID, FeeBPS: 150, PayoutFeeBPS: 100, Origin: staffer("stf_a"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, merchants.Decision{
		RequestID: req.ID, Approve: true, Origin: staffer("stf_b"),
	}); err != nil {
		t.Fatal(err)
	}
	d, err := svc.Detail(ctx, m.ID)
	if err != nil || d.FeeBPS != 150 || d.PayoutFeeBPS != 100 {
		t.Fatalf("detail = %+v, %v", d, err)
	}
	listed, _ := svc.ListRequests(ctx, merchants.RequestFilter{MerchantID: m.ID})
	if len(listed) != 1 || listed[0].MerchantName != "Soleil" {
		t.Fatalf("listed = %+v", listed)
	}
}
