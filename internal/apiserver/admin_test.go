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

// newStaff creates a super administrator the way the bootstrap command does.
func (e *env) newStaff(t *testing.T) string {
	t.Helper()
	email := strings.ToLower(ids.New("boss_")) + "@example.com"
	_, err := e.c.Staff.Create(context.Background(), staff.CreateInput{
		Email: email, Name: "The Boss", Role: staff.RoleSuperAdmin, Password: dashPhrase,
		By: audit.Actor{Kind: audit.ActorSystem, ID: "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return email
}

func (e *env) staffLogin(email, password string) response {
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
	return e.dash("POST", "/admin/session", "", body)
}

func TestAdminSignInOpensASessionThatNeedsAPasskey(t *testing.T) {
	e := newEnv(t)
	email := e.newStaff(t)
	r := e.staffLogin(email, dashPhrase)
	token, _ := r.json(t)["session_token"].(string)
	if r.status != 201 || r.json(t)["mfa"] != "enrol_required" || token == "" {
		t.Fatalf("login: %d %s", r.status, r.raw)
	}
	if r.header.Get("Cache-Control") != "no-store" {
		t.Fatal("a response carrying a session token may be cached")
	}
	me := e.dash("GET", "/admin/session", token, "")
	user, _ := me.json(t)["user"].(map[string]any)
	if me.status != 200 || user["role"] != "super_admin" || me.json(t)["passkeys"] != float64(0) {
		t.Fatalf("current: %d %s", me.status, me.raw)
	}
}

func TestAdminFailedSignInsLookTheSame(t *testing.T) {
	e := newEnv(t)
	email := e.newStaff(t)
	wrong := e.staffLogin(email, "wrong wrong wrong")
	ghost := e.staffLogin("ghost-"+email, "wrong wrong wrong")
	if wrong.status != 401 || ghost.status != 401 ||
		wrong.errorCode(t) != "invalid_credentials" || ghost.errorCode(t) != "invalid_credentials" {
		t.Fatalf("wrong: %d %s / ghost: %d %s", wrong.status, wrong.raw, ghost.status, ghost.raw)
	}
}

func TestAdminPasskeyCeremoniesStartAndRefuseGarbage(t *testing.T) {
	e := newEnv(t)
	token := e.staffLogin(e.newStaff(t), dashPhrase).json(t)["session_token"].(string)
	begin := e.dash("POST", "/admin/passkeys/begin", token, "")
	if begin.status != 201 || !strings.Contains(string(begin.raw), `"challenge"`) {
		t.Fatalf("begin: %d %s", begin.status, begin.raw)
	}
	answer := `{"label":"Key","credential":{"id":"x"}}`
	finish := e.dash("POST", "/admin/passkeys/finish", token, answer)
	if finish.status != 422 || finish.errorCode(t) != "invalid_code" {
		t.Fatalf("garbage answer: %d %s", finish.status, finish.raw)
	}
	if r := e.dash("POST", "/admin/session/passkey/begin", token, ""); r.status != 400 {
		t.Fatalf("a member with no passkeys started a passkey login: %d", r.status)
	}
	if r := e.dash("GET", "/admin/passkeys", token, ""); r.status != 200 {
		t.Fatalf("list: %d", r.status)
	}
}

func TestAdminAndMerchantSessionsAreNotInterchangeable(t *testing.T) {
	e := newEnv(t)
	merchantToken, _ := e.enrolledSession(t)
	staffToken := e.staffLogin(e.newStaff(t), dashPhrase).json(t)["session_token"].(string)
	if r := e.dash("GET", "/admin/session", merchantToken, ""); r.status != 401 {
		t.Fatalf("a merchant session opened the admin: %d", r.status)
	}
	if r := e.dash("GET", "/dashboard/session", staffToken, ""); r.status != 401 {
		t.Fatalf("a staff session opened the dashboard: %d", r.status)
	}
}

func TestAdminSignOutEndsTheSession(t *testing.T) {
	e := newEnv(t)
	token := e.staffLogin(e.newStaff(t), dashPhrase).json(t)["session_token"].(string)
	if r := e.dash("DELETE", "/admin/session", token, ""); r.status != 204 {
		t.Fatalf("logout: %d", r.status)
	}
	if r := e.dash("GET", "/admin/session", token, ""); r.status != 401 {
		t.Fatalf("a signed-out token still works: %d", r.status)
	}
}
