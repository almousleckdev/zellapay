package apiserver_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/platform/ids"
	"github.com/zellapay/zellapay/internal/staff"
)

// staffSession signs in a member with the role and completes the passkey step
// directly in the database, since a test cannot drive a real authenticator.
func (e *env) staffSession(t *testing.T, role staff.Role, passkey bool) string {
	t.Helper()
	email := strings.ToLower(ids.New("stf_")) + "@example.com"
	_, err := e.c.Staff.Create(context.Background(), staff.CreateInput{
		Email: email, Name: "Staff", Role: role, Password: dashPhrase,
		By: audit.Actor{Kind: audit.ActorSystem, ID: "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	token := e.staffLogin(email, dashPhrase).json(t)["session_token"].(string)
	if passkey {
		_, err = e.c.Pool.Exec(context.Background(),
			`UPDATE staff_sessions SET mfa_at = now() WHERE user_id = (
			   SELECT id FROM staff_users WHERE email = $1)`, email)
		if err != nil {
			t.Fatal(err)
		}
	}
	return token
}

func reason(s string) string { return fmt.Sprintf(`{"reason":%q}`, s) }

func TestAdminMerchantsNeedAPasskeySession(t *testing.T) {
	e := newEnv(t)
	if r := e.dash("GET", "/admin/merchants", "", ""); r.status != 401 {
		t.Fatalf("no token: %d", r.status)
	}
	token := e.staffSession(t, staff.RoleSuperAdmin, false)
	r := e.dash("GET", "/admin/merchants", token, "")
	if r.status != 403 || r.errorCode(t) != "mfa_required" {
		t.Fatalf("no passkey yet: %d %s", r.status, r.raw)
	}
}

func TestAdminMerchantsRolesAreEnforced(t *testing.T) {
	e := newEnv(t)
	id := e.main.merchant.ID
	support := e.staffSession(t, staff.RoleSupport, true)
	if r := e.dash("GET", "/admin/merchants/"+id, support, ""); r.status != 200 {
		t.Fatalf("support read: %d %s", r.status, r.raw)
	}
	r := e.dash("POST", "/admin/merchants/"+id+"/suspend", support, reason("abuse report"))
	if r.status != 403 || r.errorCode(t) != "forbidden" {
		t.Fatalf("support suspend: %d %s", r.status, r.raw)
	}
}

// standing posts a status change to /admin/merchants/{id}/{action}.
func (e *env) standing(token, path, why string) response {
	return e.dash("POST", "/admin/merchants/"+path, token, reason(why))
}

func TestAdminSuspendAndRestoreChangeTheStatus(t *testing.T) {
	e := newEnv(t)
	id := e.main.merchant.ID
	token := e.staffSession(t, staff.RoleRiskAnalyst, true)
	if r := e.standing(token, id+"/suspend", "no"); r.status != 400 {
		t.Fatalf("short reason: %d %s", r.status, r.raw)
	}
	r := e.standing(token, id+"/suspend", "chargeback spike")
	if r.status != 200 || r.json(t)["status"] != "suspended" {
		t.Fatalf("suspend: %d %s", r.status, r.raw)
	}
	again := e.standing(token, id+"/suspend", "second time")
	if again.status != 409 || again.errorCode(t) != "invalid_transition" {
		t.Fatalf("suspend twice: %d %s", again.status, again.raw)
	}
	r = e.standing(token, id+"/restore", "cleared after review")
	if r.status != 200 || r.json(t)["status"] != "active" {
		t.Fatalf("restore: %d %s", r.status, r.raw)
	}
}

func TestAdminStatusChangesAreAuditedWithTheReason(t *testing.T) {
	e := newEnv(t)
	id := e.main.merchant.ID
	token := e.staffSession(t, staff.RoleAdmin, true)
	e.standing(token, id+"/suspend", "chargeback spike")
	e.standing(token, id+"/restore", "cleared after review")
	recs, err := e.c.Audit.List(context.Background(),
		audit.Filter{TargetKind: "merchant", TargetID: id})
	if err != nil || len(recs) != 2 || recs[0].Action != "merchant.restored" ||
		recs[1].Reason != "chargeback spike" || recs[1].Actor.Kind != audit.ActorStaff {
		t.Fatalf("audit = %+v, %v", recs, err)
	}
}

func TestAdminMerchantListPagesAndFilters(t *testing.T) {
	e := newEnv(t)
	token := e.staffSession(t, staff.RoleSupport, true)
	r := e.dash("GET", "/admin/merchants?limit=1", token, "")
	data, _ := r.json(t)["data"].([]any)
	next, _ := r.json(t)["next_cursor"].(string)
	if r.status != 200 || len(data) != 1 || next == "" {
		t.Fatalf("page 1: %d %s", r.status, r.raw)
	}
	r = e.dash("GET", "/admin/merchants?limit=1&cursor="+next, token, "")
	if r.status != 200 {
		t.Fatalf("page 2: %d %s", r.status, r.raw)
	}
	for _, bad := range []string{"?cursor=garbage", "?status=gone", "?limit=0x"} {
		if r := e.dash("GET", "/admin/merchants"+bad, token, ""); r.status != 400 {
			t.Fatalf("%s: %d", bad, r.status)
		}
	}
	if r := e.dash("GET", "/admin/merchants/mer_missing", token, ""); r.status != 404 {
		t.Fatalf("missing: %d", r.status)
	}
}

func TestAdminSessionListsThePermissionsOfTheRole(t *testing.T) {
	e := newEnv(t)
	token := e.staffSession(t, staff.RoleSupport, true)
	perms, _ := e.dash("GET", "/admin/session", token, "").json(t)["permissions"].([]any)
	if len(perms) != 1 || perms[0] != "merchants:read" {
		t.Fatalf("permissions = %v", perms)
	}
}
