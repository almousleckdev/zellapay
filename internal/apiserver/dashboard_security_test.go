package apiserver_test

import (
	"encoding/base32"
	"fmt"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/identity"
)

const newDashPhrase = "une phrase de passe différente"

// enrolledSession signs a new member in and sets MFA up. It returns the
// session token and the authenticator secret.
func (e *env) enrolledSession(t *testing.T) (string, []byte) {
	t.Helper()
	return e.enrolledAs(t, identity.RoleOwner)
}

func (e *env) enrolledAs(t *testing.T, role identity.Role) (string, []byte) {
	t.Helper()
	m := e.newMemberAs(t, role)
	token := e.login(t, m, dashPhrase).json(t)["session_token"].(string)
	begin := e.dash("POST", "/dashboard/mfa/enrolment", token, "")
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).
		DecodeString(begin.json(t)["manual_key"].(string))
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"code":%q}`, e.code(secret))
	if r := e.dash("POST", "/dashboard/mfa/enrolment/confirm", token, body); r.status != 200 {
		t.Fatalf("confirm: %d %s", r.status, r.raw)
	}
	return token, secret
}

func (e *env) changePassword(token, current, next string) response {
	body := fmt.Sprintf(`{"current_password":%q,"new_password":%q}`, current, next)
	return e.dash("POST", "/dashboard/password/change", token, body)
}

func TestDashboardPasswordChangeNeedsARecentSecondFactor(t *testing.T) {
	e := newEnv(t)
	token, secret := e.enrolledSession(t)
	e.clock.advance(11 * time.Minute)
	r := e.changePassword(token, dashPhrase, newDashPhrase)
	if r.status != 403 || r.errorCode(t) != "step_up_required" {
		t.Fatalf("with a stale second factor: %d %s", r.status, r.raw)
	}
	answer := fmt.Sprintf(`{"code":%q}`, e.code(secret))
	if r := e.dash("POST", "/dashboard/session/mfa", token, answer); r.status != 200 {
		t.Fatalf("step-up: %d %s", r.status, r.raw)
	}
	if r := e.changePassword(token, "not the password", newDashPhrase); r.status != 401 {
		t.Fatalf("wrong current password: %d %s", r.status, r.raw)
	}
	if r := e.changePassword(token, dashPhrase, newDashPhrase); r.status != 204 {
		t.Fatalf("after step-up: %d %s", r.status, r.raw)
	}
}

func TestDashboardRegeneratesRecoveryCodes(t *testing.T) {
	e := newEnv(t)
	token, _ := e.enrolledSession(t)
	r := e.dash("POST", "/dashboard/mfa/recovery-codes", token, "")
	codes, _ := r.json(t)["recovery_codes"].([]any)
	if r.status != 200 || len(codes) != 10 {
		t.Fatalf("regenerate: %d %s", r.status, r.raw)
	}
}

func TestDashboardRemovesTheAuthenticatorAndAsksToEnrolAgain(t *testing.T) {
	e := newEnv(t)
	token, _ := e.enrolledSession(t)
	if r := e.dash("DELETE", "/dashboard/mfa", token, ""); r.status != 204 {
		t.Fatalf("remove: %d %s", r.status, r.raw)
	}
	me := e.dash("GET", "/dashboard/session", token, "")
	if me.json(t)["mfa"] != "enrol_required" {
		t.Fatalf("after removal: %s", me.raw)
	}
	if r := e.dash("POST", "/dashboard/mfa/recovery-codes", token, ""); r.status != 403 {
		t.Fatalf("recovery codes without an authenticator: %d", r.status)
	}
	if r := e.dash("POST", "/dashboard/mfa/enrolment", token, ""); r.status != 201 {
		t.Fatalf("enrolling again: %d %s", r.status, r.raw)
	}
}
