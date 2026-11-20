package apiserver_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/platform/ids"
	"github.com/zellapay/zellapay/internal/staff"
)

func newMerchantBody(name, currency, why string) string {
	return fmt.Sprintf(`{"name":%q,"currency":%q,"reason":%q}`, name, currency, why)
}

func TestAdminCreatesAPendingMerchantWithAnAuditedReason(t *testing.T) {
	e := newEnv(t)
	token := e.staffSession(t, staff.RoleAdmin, true)
	body := newMerchantBody("Boutique Soleil", "XAF", "signed contract 2026-114")
	r := e.dash("POST", "/admin/merchants", token, body)
	id, _ := r.json(t)["id"].(string)
	if r.status != 201 || r.json(t)["status"] != "pending" || id == "" {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	recs, err := e.c.Audit.List(context.Background(),
		audit.Filter{TargetKind: "merchant", TargetID: id})
	if err != nil || len(recs) != 1 || recs[0].Action != "merchant.created" ||
		recs[0].Reason != "signed contract 2026-114" {
		t.Fatalf("audit = %+v, %v", recs, err)
	}
}

func TestAdminCreateIsRefusedForBadInputAndWeakRoles(t *testing.T) {
	e := newEnv(t)
	admin := e.staffSession(t, staff.RoleAdmin, true)
	for _, body := range []string{
		newMerchantBody("", "XAF", "valid reason"),
		newMerchantBody("Shop", "xaf", "valid reason"),
		newMerchantBody("Shop", "XAF", "no"),
		`{"name":"Shop","currency":"XAF","reason":"valid reason","extra":1}`,
	} {
		if r := e.dash("POST", "/admin/merchants", admin, body); r.status != 400 {
			t.Fatalf("%s: %d %s", body, r.status, r.raw)
		}
	}
	risk := e.staffSession(t, staff.RoleRiskAnalyst, true)
	body := newMerchantBody("Shop", "XAF", "valid reason")
	r := e.dash("POST", "/admin/merchants", risk, body)
	if r.status != 403 || r.errorCode(t) != "forbidden" {
		t.Fatalf("risk analyst create: %d %s", r.status, r.raw)
	}
}

func inviteBody(email, role, why string) string {
	return fmt.Sprintf(`{"email":%q,"role":%q,"locale":"en","reason":%q}`, email, role, why)
}

func TestAdminInvitesAnOwnerAndAuditsIt(t *testing.T) {
	e := newEnv(t)
	token := e.staffSession(t, staff.RoleAdmin, true)
	id := e.main.merchant.ID
	email := ids.New("owner_") + "@example.com"
	r := e.dash("POST", "/admin/merchants/"+id+"/invitations", token,
		inviteBody(email, "owner", "contract signed"))
	if r.status != 201 || r.json(t)["role"] != "owner" || r.json(t)["id"] == "" {
		t.Fatalf("invite: %d %s", r.status, r.raw)
	}
	recs, err := e.c.Audit.List(context.Background(), audit.Filter{Action: "invitation.created"})
	if err != nil || len(recs) == 0 || recs[0].Reason != "contract signed" ||
		recs[0].Actor.Kind != audit.ActorStaff {
		t.Fatalf("audit = %+v, %v", recs, err)
	}
}

func TestAdminInviteRefusals(t *testing.T) {
	e := newEnv(t)
	admin := e.staffSession(t, staff.RoleAdmin, true)
	path := "/admin/merchants/" + e.main.merchant.ID + "/invitations"
	post := func(token, path, email, role string) response {
		return e.dash("POST", path, token, inviteBody(email, role, "valid reason"))
	}
	if r := post(admin, path, "not-an-email", "owner"); r.status != 400 {
		t.Fatalf("bad email: %d %s", r.status, r.raw)
	}
	if r := post(admin, path, "a@example.com", "emperor"); r.status != 400 {
		t.Fatalf("bad role: %d %s", r.status, r.raw)
	}
	r := post(admin, "/admin/merchants/mer_missing/invitations", "a@example.com", "owner")
	if r.status != 404 || r.errorCode(t) != "not_found" {
		t.Fatalf("unknown merchant: %d %s", r.status, r.raw)
	}
	support := e.staffSession(t, staff.RoleSupport, true)
	if r := post(support, path, "a@example.com", "owner"); r.status != 403 {
		t.Fatalf("support invite: %d", r.status)
	}
}

func TestAdminOverviewCountsMerchants(t *testing.T) {
	e := newEnv(t)
	token := e.staffSession(t, staff.RoleSupport, true)
	r := e.dash("GET", "/admin/overview?days=7", token, "")
	days, _ := r.json(t)["created_by_day"].([]any)
	by, _ := r.json(t)["by_status"].(map[string]any)
	total, _ := r.json(t)["total"].(float64)
	if r.status != 200 || len(days) != 7 || total < 2 || by["active"] == nil {
		t.Fatalf("overview: %d %s", r.status, r.raw)
	}
	for _, bad := range []string{"?days=0", "?days=91", "?days=x"} {
		if r := e.dash("GET", "/admin/overview"+bad, token, ""); r.status != 400 {
			t.Fatalf("%s: %d", bad, r.status)
		}
	}
}
