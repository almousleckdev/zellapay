package pgstore_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/staff"
)

var src = identity.Source{IP: "198.51.100.7", UserAgent: "test"}

func (w *world) login(email, password string) (staff.Issued, error) {
	in := staff.LoginInput{Email: email, Password: password, Source: src}
	return w.svc.Login(context.Background(), in)
}

func TestCreateStoresAHashedPasswordAndAuditsIt(t *testing.T) {
	w := newWorld(t)
	u := w.superAdmin(t)
	if u.Email != "boss@example.com" || u.Role != staff.RoleSuperAdmin {
		t.Fatalf("user = %+v", u)
	}
	var hash string
	row := w.pool.QueryRow(context.Background(), "SELECT password_hash FROM staff_users")
	if err := row.Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") || strings.Contains(hash, "tournesol") {
		t.Fatalf("password hash = %q", hash)
	}
	if n := w.count(t, "SELECT count(*) FROM audit_log WHERE action = 'staff.created'"); n != 1 {
		t.Fatalf("audit entries = %d", n)
	}
}

func TestCreateRefusesBadInputAndDuplicates(t *testing.T) {
	w := newWorld(t)
	w.superAdmin(t)
	by := audit.Actor{Kind: audit.ActorSystem, ID: "zellactl"}
	admin := staff.RoleAdmin
	cases := map[string]staff.CreateInput{
		"weak password": {Email: "a@b.ga", Name: "A", Role: admin, Password: "short", By: by},
		"bad role":      {Email: "a@b.ga", Name: "A", Role: "king", Password: phrase, By: by},
		"bad email":     {Email: "nope", Name: "A", Role: admin, Password: phrase, By: by},
	}
	for name, in := range cases {
		if _, err := w.svc.Create(context.Background(), in); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	dup := staff.CreateInput{
		Email: "BOSS@example.com", Name: "B", Role: admin, Password: phrase, By: by,
	}
	if _, err := w.svc.Create(context.Background(), dup); !errors.Is(err, identity.ErrEmailTaken) {
		t.Fatalf("duplicate: %v", err)
	}
}

func TestLoginOpensASessionThatStillNeedsAPasskey(t *testing.T) {
	w := newWorld(t)
	w.superAdmin(t)
	s, err := w.login("boss@example.com", phrase)
	if err != nil || s.MFA != staff.MFAEnrolRequired || len(s.Token) < 43 {
		t.Fatalf("issued = %+v, %v", s, err)
	}
	p, err := w.svc.Resolve(context.Background(), s.Token)
	if err != nil || p.User.Role != staff.RoleSuperAdmin || p.MFA != staff.MFAEnrolRequired {
		t.Fatalf("principal = %+v, %v", p, err)
	}
}

func TestEveryFailedLoginLooksTheSame(t *testing.T) {
	w := newWorld(t)
	w.superAdmin(t)
	for _, tc := range [][2]string{
		{"boss@example.com", "wrong wrong wrong"}, {"ghost@example.com", phrase}, {"nope", phrase},
	} {
		if _, err := w.login(tc[0], tc[1]); !errors.Is(err, identity.ErrInvalidCredentials) {
			t.Errorf("%v: err = %v", tc, err)
		}
	}
	w.exec(t, "UPDATE staff_users SET status = 'disabled'")
	_, err := w.login("boss@example.com", phrase)
	if !errors.Is(err, identity.ErrInvalidCredentials) {
		t.Fatalf("a disabled account signed in: %v", err)
	}
}

func (w *world) exec(t *testing.T, sql string) {
	t.Helper()
	if _, err := w.pool.Exec(context.Background(), sql); err != nil {
		t.Fatal(err)
	}
}

func TestStaffSessionsAreShorterThanMerchants(t *testing.T) {
	w := newWorld(t)
	w.superAdmin(t)
	ctx := context.Background()
	idle, _ := w.login("boss@example.com", phrase)
	w.clock.advance(16 * time.Minute)
	if _, err := w.svc.Resolve(ctx, idle.Token); !errors.Is(err, identity.ErrInvalidSession) {
		t.Fatalf("idle for 16 minutes: %v", err)
	}
	busy, _ := w.login("boss@example.com", phrase)
	for range 36 {
		w.clock.advance(14 * time.Minute)
		_, _ = w.svc.Resolve(ctx, busy.Token)
	}
	if _, err := w.svc.Resolve(ctx, busy.Token); !errors.Is(err, identity.ErrInvalidSession) {
		t.Fatalf("a session outlived its 8 hours: %v", err)
	}
}

func TestLogoutAndUnknownTokensLookTheSame(t *testing.T) {
	w := newWorld(t)
	w.superAdmin(t)
	ctx := context.Background()
	s, _ := w.login("boss@example.com", phrase)
	if err := w.svc.Logout(ctx, s.Token); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{s.Token, "nonsense"} {
		if _, err := w.svc.Resolve(ctx, token); !errors.Is(err, identity.ErrInvalidSession) {
			t.Errorf("%q: %v", token, err)
		}
	}
}

func TestPasskeyCeremoniesBeginAndRefuseGarbageOnce(t *testing.T) {
	w := newWorld(t)
	w.superAdmin(t)
	ctx := context.Background()
	s, _ := w.login("boss@example.com", phrase)
	options, err := w.svc.BeginRegistration(ctx, s.Token)
	text := string(options)
	hasBoth := strings.Contains(text, `"challenge"`) && strings.Contains(text, `"localhost"`)
	if err != nil || !hasBoth {
		t.Fatalf("options = %s, %v", options, err)
	}
	in := staff.FinishInput{Token: s.Token, Body: []byte(`{"id":"x"}`), Label: "Key"}
	if _, err := w.svc.FinishRegistration(ctx, in); !errors.Is(err, identity.ErrInvalidCode) {
		t.Fatalf("garbage answer: %v", err)
	}
	if _, err := w.svc.FinishRegistration(ctx, in); !errors.Is(err, identity.ErrInvalidCode) {
		t.Fatalf("a spent challenge was accepted again: %v", err)
	}
	if p, _ := w.svc.Resolve(ctx, s.Token); p.MFA != staff.MFAEnrolRequired {
		t.Fatal("a garbage answer completed enrolment")
	}
	if _, err := w.svc.BeginLogin(ctx, s.Token); err == nil {
		t.Fatal("a session with no passkeys started a passkey login")
	}
}
