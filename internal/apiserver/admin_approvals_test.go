package apiserver_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/zellapay/zellapay/internal/platform/ids"
	"github.com/zellapay/zellapay/internal/staff"
)

func (e *env) pendingMerchant(t *testing.T) string {
	t.Helper()
	m, err := e.c.Merchants.CreateMerchant(context.Background(), "Soleil "+ids.New(""), "XAF")
	if err != nil {
		t.Fatal(err)
	}
	return m.ID
}

func (e *env) decision(token, id, verb string) response {
	return e.dash("POST", "/admin/approvals/"+id+"/"+verb, token, reason("checked the papers"))
}

func TestActivationNeedsASecondStaffMemberToApprove(t *testing.T) {
	e := newEnv(t)
	id := e.pendingMerchant(t)
	asker := e.staffSession(t, staff.RoleRiskAnalyst, true)
	r := e.dash("POST", "/admin/merchants/"+id+"/activation", asker, reason("documents verified"))
	reqID, _ := r.json(t)["id"].(string)
	body := r.json(t)
	if r.status != 201 || body["status"] != "pending" || body["requested_by_you"] != true {
		t.Fatalf("request: %d %s", r.status, r.raw)
	}
	path := "/admin/merchants/" + id + "/activation"
	if again := e.dash("POST", path, asker, reason("one more time")); again.status != 409 {
		t.Fatalf("second open request: %d %s", again.status, again.raw)
	}
	if r := e.decision(asker, reqID, "approve"); r.status != 403 {
		t.Fatalf("a risk analyst may not decide: %d %s", r.status, r.raw)
	}
	boss := e.staffSession(t, staff.RoleSuperAdmin, true)
	r = e.decision(boss, reqID, "approve")
	if r.status != 200 || r.json(t)["status"] != "approved" {
		t.Fatalf("approve: %d %s", r.status, r.raw)
	}
	d := e.dash("GET", "/admin/merchants/"+id, boss, "")
	if d.json(t)["status"] != "active" {
		t.Fatalf("merchant = %s", d.raw)
	}
}

func TestTheRequesterCannotApproveEvenWithTheRole(t *testing.T) {
	e := newEnv(t)
	id := e.pendingMerchant(t)
	boss := e.staffSession(t, staff.RoleSuperAdmin, true)
	r := e.dash("POST", "/admin/merchants/"+id+"/activation", boss, reason("documents verified"))
	reqID, _ := r.json(t)["id"].(string)
	own := e.decision(boss, reqID, "approve")
	if own.status != 403 || own.errorCode(t) != "self_approval" {
		t.Fatalf("self approval: %d %s", own.status, own.raw)
	}
	if got := e.dash("GET", "/admin/merchants/"+id, boss, ""); got.json(t)["status"] != "pending" {
		t.Fatalf("merchant changed: %s", got.raw)
	}
}

func TestFeeChangeIsAppliedOnlyAfterApprovalAndRejectionChangesNothing(t *testing.T) {
	e := newEnv(t)
	id := e.pendingMerchant(t)
	asker := e.staffSession(t, staff.RoleAdmin, true)
	body := fmt.Sprintf(`{"fee_bps":150,"payout_fee_bps":100,"reason":%q}`, "new contract signed")
	r := e.dash("POST", "/admin/merchants/"+id+"/fees", asker, body)
	reqID, _ := r.json(t)["id"].(string)
	if r.status != 201 {
		t.Fatalf("request: %d %s", r.status, r.raw)
	}
	before := e.dash("GET", "/admin/merchants/"+id, asker, "").json(t)["fee_bps"]
	if before != float64(0) {
		t.Fatalf("fee changed before approval: %v", before)
	}
	boss := e.staffSession(t, staff.RoleSuperAdmin, true)
	r = e.decision(boss, reqID, "reject")
	if r.status != 200 || r.json(t)["status"] != "rejected" {
		t.Fatalf("reject: %d %s", r.status, r.raw)
	}
	got := e.dash("GET", "/admin/merchants/"+id, boss, "").json(t)["fee_bps"]
	if got != float64(0) {
		t.Fatalf("a rejection changed the fee: %v", got)
	}
}

func TestFeeRequestsAreValidatedAndRoleLimited(t *testing.T) {
	e := newEnv(t)
	id := e.main.merchant.ID
	asker := e.staffSession(t, staff.RoleAdmin, true)
	bad := `{"fee_bps":10001,"payout_fee_bps":0,"reason":"far too high"}`
	if r := e.dash("POST", "/admin/merchants/"+id+"/fees", asker, bad); r.status != 400 {
		t.Fatalf("bad fee: %d %s", r.status, r.raw)
	}
	risk := e.staffSession(t, staff.RoleRiskAnalyst, true)
	ok := `{"fee_bps":100,"payout_fee_bps":100,"reason":"new contract"}`
	if r := e.dash("POST", "/admin/merchants/"+id+"/fees", risk, ok); r.status != 403 {
		t.Fatalf("risk analyst fee request: %d", r.status)
	}
}

func TestApprovalsListActivityAndOperators(t *testing.T) {
	e := newEnv(t)
	id := e.pendingMerchant(t)
	asker := e.staffSession(t, staff.RoleAdmin, true)
	e.dash("POST", "/admin/merchants/"+id+"/activation", asker, reason("documents verified"))
	list := e.dash("GET", "/admin/approvals?status=pending&merchant_id="+id, asker, "")
	data, _ := list.json(t)["data"].([]any)
	if list.status != 200 || len(data) != 1 {
		t.Fatalf("list: %d %s", list.status, list.raw)
	}
	act := e.dash("GET", "/admin/merchants/"+id+"/activity", asker, "")
	entries, _ := act.json(t)["data"].([]any)
	if act.status != 200 || len(entries) != 1 {
		t.Fatalf("activity: %d %s", act.status, act.raw)
	}
	first, _ := entries[0].(map[string]any)
	if first["action"] != "merchant.activation_requested" || first["actor"] != "Staff" {
		t.Fatalf("entry = %v", first)
	}
	ops := e.dash("GET", "/admin/operators", asker, "")
	rows, _ := ops.json(t)["data"].([]any)
	if ops.status != 200 || len(rows) == 0 {
		t.Fatalf("operators: %d %s", ops.status, ops.raw)
	}
	ov := e.dash("GET", "/admin/overview?days=7", asker, "").json(t)
	if n, _ := ov["pending_approvals"].(float64); n < 1 {
		t.Fatalf("overview pending_approvals = %v", ov["pending_approvals"])
	}
}

func TestApprovedFeeChangeShowsOnTheMerchant(t *testing.T) {
	e := newEnv(t)
	id := e.pendingMerchant(t)
	asker := e.staffSession(t, staff.RoleAdmin, true)
	body := `{"fee_bps":150,"payout_fee_bps":100,"reason":"new contract signed"}`
	reqID, _ := e.dash("POST", "/admin/merchants/"+id+"/fees", asker, body).json(t)["id"].(string)
	boss := e.staffSession(t, staff.RoleSuperAdmin, true)
	if r := e.decision(boss, reqID, "approve"); r.status != 200 {
		t.Fatalf("approve: %d %s", r.status, r.raw)
	}
	got := e.dash("GET", "/admin/merchants/"+id, boss, "").json(t)
	if got["fee_bps"] != float64(150) || got["payout_fee_bps"] != float64(100) {
		t.Fatalf("merchant = %v", got)
	}
}
