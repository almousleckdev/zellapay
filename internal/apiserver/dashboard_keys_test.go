package apiserver_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/identity"
)

func (e *env) createKey(token, label string, scopes ...string) response {
	list := ""
	for i, s := range scopes {
		if i > 0 {
			list += ","
		}
		list += fmt.Sprintf("%q", s)
	}
	body := fmt.Sprintf(`{"label":%q,"scopes":[%s]}`, label, list)
	return e.dash("POST", "/dashboard/api-keys", token, body)
}

// issued is a key as the dashboard returned it.
type issued struct{ id, secret string }

func (e *env) issueKey(t *testing.T, token string) issued {
	t.Helper()
	created := e.createKey(token, "Website", "read")
	key, _ := created.json(t)["key"].(map[string]any)
	got := issued{id: fmt.Sprint(key["id"]), secret: fmt.Sprint(created.json(t)["secret"])}
	if created.status != 201 || key["label"] != "Website" || key["mode"] != "test" {
		t.Fatalf("create: %d %s", created.status, created.raw)
	}
	return got
}

func (e *env) rotateKey(t *testing.T, token, id string) issued {
	t.Helper()
	r := e.dash("POST", "/dashboard/api-keys/"+id+"/rotate", token, `{"overlap_hours":1}`)
	key, _ := r.json(t)["key"].(map[string]any)
	if r.status != 201 {
		t.Fatalf("rotate: %d %s", r.status, r.raw)
	}
	return issued{id: fmt.Sprint(key["id"]), secret: fmt.Sprint(r.json(t)["secret"])}
}

// expectKey checks whether a secret is accepted by the merchant API.
func (e *env) expectKey(t *testing.T, secret string, status int, why string) {
	t.Helper()
	if got := e.get("/v1/collections", secret).status; got != status {
		t.Fatalf("%s: /v1 answered %d, want %d", why, got, status)
	}
}

func TestDashboardKeyLifecycle(t *testing.T) {
	e := newEnv(t)
	token, _ := e.enrolledSession(t)
	first := e.issueKey(t, token)
	e.expectKey(t, first.secret, 200, "the new key")
	listed := e.dash("GET", "/dashboard/api-keys", token, "")
	if listed.status != 200 || containsSecret(listed.raw, first.secret) {
		t.Fatalf("list: %d (a secret in a list would be a leak)", listed.status)
	}
	second := e.rotateKey(t, token, first.id)
	if second.secret == first.secret {
		t.Fatal("rotation returned the same secret")
	}
	e.expectKey(t, first.secret, 200, "the old key during the overlap")
	e.expectKey(t, second.secret, 200, "the replacement")
	if r := e.dash("DELETE", "/dashboard/api-keys/"+second.id, token, ""); r.status != 204 {
		t.Fatalf("revoke: %d %s", r.status, r.raw)
	}
	e.expectKey(t, second.secret, 401, "a revoked key")
	e.clock.advance(2 * time.Hour)
	e.expectKey(t, first.secret, 401, "the old key after the overlap")
}

func containsSecret(body []byte, secret string) bool {
	return len(secret) > 0 && strings.Contains(string(body), secret)
}

func TestDashboardKeysNeedARecentSecondFactor(t *testing.T) {
	e := newEnv(t)
	token, _ := e.enrolledSession(t)
	e.clock.advance(11 * time.Minute)
	r := e.createKey(token, "Late", "read")
	if r.status != 403 || r.errorCode(t) != "step_up_required" {
		t.Fatalf("create with a stale factor: %d %s", r.status, r.raw)
	}
	if e.dash("GET", "/dashboard/api-keys", token, "").status != 200 {
		t.Fatal("listing keys needs no step-up")
	}
}

func TestDashboardAKeyCannotOutrankItsCreator(t *testing.T) {
	e := newEnv(t)
	token, _ := e.enrolledAs(t, identity.RoleDeveloper)
	if r := e.createKey(token, "Money", "payouts:write"); r.status != 403 {
		t.Fatalf("a developer minted a payouts key: %d %s", r.status, r.raw)
	}
	if r := e.createKey(token, "Hooks", "webhooks:write"); r.status != 201 {
		t.Fatalf("a developer should manage webhook keys: %d %s", r.status, r.raw)
	}
	if r := e.createKey(token, "Odd", "admin"); r.status != 400 {
		t.Fatalf("an unknown scope: %d %s", r.status, r.raw)
	}
	support, _ := e.enrolledAs(t, identity.RoleSupport)
	if r := e.dash("GET", "/dashboard/api-keys", support, ""); r.status != 403 {
		t.Fatalf("support listed keys: %d", r.status)
	}
}

func TestDashboardKeyChangesAreAudited(t *testing.T) {
	e := newEnv(t)
	token, _ := e.enrolledSession(t)
	created := e.createKey(token, "Audited", "read")
	id, _ := created.json(t)["key"].(map[string]any)["id"].(string)
	e.dash("DELETE", "/dashboard/api-keys/"+id, token, "")
	var n int
	err := e.c.Pool.QueryRow(contextBackground(),
		`SELECT count(*) FROM audit_log WHERE target_id = $1
		   AND action IN ('api_key.created', 'api_key.revoked')`, id).Scan(&n)
	if err != nil || n != 2 {
		t.Fatalf("audit entries = %d, %v", n, err)
	}
}

func TestDashboardOneMerchantCannotTouchAnothersKey(t *testing.T) {
	e := newEnv(t)
	token, _ := e.enrolledSession(t)
	foreign := e.issue(e.other.merchant, "read")
	_ = foreign
	keys, _ := e.c.Merchants.ListKeys(contextBackground(), e.other.merchant.ID)
	if r := e.dash("DELETE", "/dashboard/api-keys/"+keys[0].ID, token, ""); r.status != 404 {
		t.Fatalf("revoking another merchant's key: %d %s", r.status, r.raw)
	}
}
