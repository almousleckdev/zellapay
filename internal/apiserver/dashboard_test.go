package apiserver_test

import (
	"encoding/base32"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/ids"
	"github.com/zellapay/zellapay/internal/platform/totp"
)

const dashPhrase = "tournesol bleu sur la table"

// member is a user created through the real invitation flow.
type member struct {
	email string
}

func (e *env) newMember(t *testing.T) member {
	t.Helper()
	email := strings.ToLower(ids.New("jane_")) + "@example.com"
	inv, err := e.c.Identity.Invite(contextBackground(), identity.InviteInput{
		MerchantID: e.main.merchant.ID, Email: email, Role: identity.RoleOwner,
		InvitedBy: audit.Actor{Kind: audit.ActorSystem, ID: "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	token := "test-token-" + inv.ID + strings.Repeat("x", 20)
	_, err = e.c.Pool.Exec(contextBackground(),
		"UPDATE invitations SET token_hash = sha256($1::bytea) WHERE id = $2", token, inv.ID)
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"token":%q,"name":"Jane","password":%q}`, token, dashPhrase)
	if r := e.dash("POST", "/dashboard/invitations/accept", "", body); r.status != 201 {
		t.Fatalf("accept: %d %s", r.status, r.raw)
	}
	return member{email: email}
}

func (e *env) dash(method, path, token, body string) response {
	return e.do(request{method: method, path: path, key: token, body: body})
}

func (e *env) login(t *testing.T, m member, password string) response {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, m.email, password)
	return e.dash("POST", "/dashboard/session", "", body)
}

func (e *env) code(secret []byte) string {
	return totp.Code(secret, totp.Step(e.clock.Now()))
}

// enrol sets up MFA for the session behind token. It returns the codes.
func (e *env) enrol(t *testing.T, token string) []any {
	t.Helper()
	begin := e.dash("POST", "/dashboard/mfa/enrolment", token, "")
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).
		DecodeString(begin.json(t)["manual_key"].(string))
	if begin.status != 201 || err != nil {
		t.Fatalf("begin: %d %s", begin.status, begin.raw)
	}
	done := e.dash("POST", "/dashboard/mfa/enrolment/confirm", token,
		fmt.Sprintf(`{"code":%q}`, e.code(secret)))
	codes, _ := done.json(t)["recovery_codes"].([]any)
	if done.status != 200 {
		t.Fatalf("confirm: %d %s", done.status, done.raw)
	}
	return codes
}

// expectOwnerWithMFA checks the session is complete and may manage sessions.
func (e *env) expectOwnerWithMFA(t *testing.T, token string) {
	t.Helper()
	me := e.dash("GET", "/dashboard/session", token, "")
	user, _ := me.json(t)["user"].(map[string]any)
	if me.status != 200 || me.json(t)["mfa"] != "done" || user["role"] != "owner" {
		t.Fatalf("current: %d %s", me.status, me.raw)
	}
	if e.dash("GET", "/dashboard/sessions", token, "").status != 200 {
		t.Fatal("a session that proved MFA cannot list sessions")
	}
}

func TestDashboardSignInEnrolmentAndSessions(t *testing.T) {
	e := newEnv(t)
	m := e.newMember(t)
	first := e.login(t, m, dashPhrase)
	if first.status != 201 || first.json(t)["mfa"] != "enrol_required" {
		t.Fatalf("login: %d %s", first.status, first.raw)
	}
	if first.header.Get("Cache-Control") != "no-store" {
		t.Fatal("a response carrying a session token may be cached")
	}
	token, _ := first.json(t)["session_token"].(string)
	if codes := e.enrol(t, token); len(codes) != 10 {
		t.Fatalf("recovery codes = %d", len(codes))
	}
	e.expectOwnerWithMFA(t, token)
	if r := e.dash("DELETE", "/dashboard/session", token, ""); r.status != 204 {
		t.Fatalf("logout: %d", r.status)
	}
	if r := e.dash("GET", "/dashboard/session", token, ""); r.status != 401 {
		t.Fatalf("a logged-out token still works: %d", r.status)
	}
}

func TestDashboardSecondFactorGatesTheSecuritySettings(t *testing.T) {
	e := newEnv(t)
	m := e.newMember(t)
	token := e.login(t, m, dashPhrase).json(t)["session_token"].(string)
	if r := e.dash("GET", "/dashboard/sessions", token, ""); r.status != 403 ||
		r.errorCode(t) != "mfa_required" {
		t.Fatalf("before MFA: %d %s", r.status, r.raw)
	}
	r := e.dash("POST", "/dashboard/session/mfa", token, `{"code":"123456"}`)
	if r.status != 400 {
		t.Fatalf("MFA not pending should be refused: %d %s", r.status, r.raw)
	}
}

func TestDashboardAnswersWrongCredentialsAlike(t *testing.T) {
	e := newEnv(t)
	m := e.newMember(t)
	wrong := e.login(t, m, "wrong wrong wrong")
	ghost := e.login(t, member{email: "ghost-" + m.email}, "wrong wrong wrong")
	if wrong.status != 401 || ghost.status != 401 || string(wrong.raw) == "" ||
		wrong.errorCode(t) != "invalid_credentials" || ghost.errorCode(t) != "invalid_credentials" {
		t.Fatalf("wrong: %d %s / ghost: %d %s", wrong.status, wrong.raw, ghost.status, ghost.raw)
	}
}

func TestDashboardSlowsDownGuessing(t *testing.T) {
	e := newEnv(t)
	m := e.newMember(t)
	var last response
	for range 6 {
		last = e.login(t, m, "wrong wrong wrong")
	}
	if last.status != 429 || last.errorCode(t) != "too_many_attempts" {
		t.Fatalf("after 6 failures: %d %s", last.status, last.raw)
	}
	if last.header.Get("Retry-After") == "" {
		t.Fatal("no Retry-After on a throttled response")
	}
	if r := e.login(t, m, dashPhrase); r.status != 429 {
		t.Fatalf("the right password got through the lockout: %d", r.status)
	}
	e.clock.advance(2 * time.Minute)
	if r := e.login(t, m, dashPhrase); r.status != 201 {
		t.Fatalf("after the delay: %d %s", r.status, r.raw)
	}
}

func TestDashboardRefusesBadRequests(t *testing.T) {
	e := newEnv(t)
	for name, tc := range map[string]struct{ path, token, body string }{
		"unknown field": {"/dashboard/session", "", `{"email":"a@b.ga","password":"x","a":1}`},
		"not json":      {"/dashboard/session", "", `nope`},
		"bad link":      {"/dashboard/email/verify", "", `{"token":"nonsense"}`},
		"no token":      {"/dashboard/sessions/end-others", "", ``},
		"unknown token": {"/dashboard/sessions/end-others", "nonsense", ``},
	} {
		r := e.dash("POST", tc.path, tc.token, tc.body)
		if r.status < 400 || r.status >= 500 || r.errorCode(t) == "" {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
}

func TestDashboardPasswordResetDoesNotRevealAccounts(t *testing.T) {
	e := newEnv(t)
	m := e.newMember(t)
	const path = "/dashboard/password/reset-request"
	known := e.dash("POST", path, "", fmt.Sprintf(`{"email":%q}`, m.email))
	ghost := e.dash("POST", path, "", `{"email":"ghost@example.com"}`)
	if known.status != 202 || ghost.status != 202 || string(known.raw) != string(ghost.raw) {
		t.Fatalf("known %d %q, ghost %d %q", known.status, known.raw, ghost.status, ghost.raw)
	}
}
