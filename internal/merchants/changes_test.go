package merchants

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/audit"
)

var approver = audit.Actor{Kind: audit.ActorStaff, ID: "stf_2"}

func pendingMerchant(t *testing.T, e env) string {
	t.Helper()
	m, err := e.svc.CreateMerchant(context.Background(), "Soleil", "XAF")
	if err != nil {
		t.Fatal(err)
	}
	return m.ID
}

func origin(by audit.Actor, why string) Origin { return Origin{By: by, Reason: why} }

func (e env) askActivation(id string, by audit.Actor, why string) (ChangeRequest, error) {
	return e.svc.RequestActivation(context.Background(), id, origin(by, why))
}

func (e env) decide(id string, approve bool, by audit.Actor, why string) (ChangeRequest, error) {
	return e.svc.Decide(context.Background(), Decision{
		RequestID: id, Approve: approve, Origin: origin(by, why),
	})
}

func TestActivationNeedsAPendingMerchantAndOneOpenRequest(t *testing.T) {
	e := auditedEnv(t)
	_, err := e.askActivation(e.mer.ID, staffer, "already live")
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("active merchant = %v", err)
	}
	id := pendingMerchant(t, e)
	r, err := e.askActivation(id, staffer, "documents checked")
	if err != nil || r.Status != RequestPending || r.Kind != ChangeActivate {
		t.Fatalf("request = %+v, %v", r, err)
	}
	if _, err := e.askActivation(id, staffer, "again please"); !errors.Is(err, ErrRequestOpen) {
		t.Fatalf("second request = %v", err)
	}
}

func TestTheRequesterCannotApproveTheirOwnRequest(t *testing.T) {
	e := auditedEnv(t)
	id := pendingMerchant(t, e)
	r, _ := e.askActivation(id, staffer, "documents checked")
	if _, err := e.decide(r.ID, true, staffer, "I agree"); !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("self approval = %v", err)
	}
	if m, _ := e.svc.GetMerchant(context.Background(), id); m.Status != StatusPending {
		t.Fatalf("a refused approval changed the merchant: %s", m.Status)
	}
}

func TestASecondApproverActivatesAndItIsAudited(t *testing.T) {
	e := auditedEnv(t)
	id := pendingMerchant(t, e)
	r, _ := e.askActivation(id, staffer, "documents checked")
	got, err := e.decide(r.ID, true, approver, "checked too")
	if err != nil || got.Status != RequestApproved || *got.DecidedBy != approver.ID {
		t.Fatalf("decide = %+v, %v", got, err)
	}
	if m, _ := e.svc.GetMerchant(context.Background(), id); m.Status != StatusActive {
		t.Fatalf("status = %s", m.Status)
	}
	last := e.repo.audits[len(e.repo.audits)-1]
	if last.Action != "merchant.activated" || last.Actor.ID != approver.ID ||
		last.Reason != "checked too" {
		t.Fatalf("audit = %+v", last)
	}
	if _, err := e.decide(r.ID, true, approver, "twice"); !errors.Is(err, ErrRequestClosed) {
		t.Fatalf("decided twice = %v", err)
	}
}

func TestRejectingChangesNothingAndAnExpiredRequestIsClosed(t *testing.T) {
	e := auditedEnv(t)
	id := pendingMerchant(t, e)
	r, _ := e.askActivation(id, staffer, "documents checked")
	rej, err := e.decide(r.ID, false, approver, "papers incomplete")
	if err != nil || rej.Status != RequestRejected {
		t.Fatalf("reject = %+v, %v", rej, err)
	}
	if m, _ := e.svc.GetMerchant(context.Background(), id); m.Status != StatusPending {
		t.Fatalf("rejecting changed the merchant: %s", m.Status)
	}
	again, _ := e.askActivation(id, staffer, "papers now complete")
	e.clock.now = e.clock.now.Add(25 * time.Hour)
	if _, err := e.decide(again.ID, true, approver, "late"); !errors.Is(err, ErrRequestClosed) {
		t.Fatalf("expired = %v", err)
	}
}

func TestFeeChangeIsValidatedAndAppliedOnApproval(t *testing.T) {
	e := auditedEnv(t)
	ctx := context.Background()
	bad := FeeChange{MerchantID: e.mer.ID, FeeBPS: 10_001, Origin: origin(staffer, "too high")}
	if _, err := e.svc.RequestFeeChange(ctx, bad); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bad fee = %v", err)
	}
	ok := FeeChange{
		MerchantID: e.mer.ID, FeeBPS: 150, PayoutFeeBPS: 100,
		Origin: origin(staffer, "new contract"),
	}
	r, err := e.svc.RequestFeeChange(ctx, ok)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.decide(r.ID, true, approver, "matches contract"); err != nil {
		t.Fatal(err)
	}
	d, _ := e.svc.Detail(ctx, e.mer.ID)
	if d.FeeBPS != 150 || d.PayoutFeeBPS != 100 {
		t.Fatalf("fees = %d / %d", d.FeeBPS, d.PayoutFeeBPS)
	}
	if n, _ := e.svc.PendingRequestCount(ctx); n != 0 {
		t.Fatalf("pending = %d", n)
	}
}

func TestADecisionNeedsAReason(t *testing.T) {
	e := auditedEnv(t)
	if _, err := e.decide("chg_x", false, approver, " "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v", err)
	}
}
