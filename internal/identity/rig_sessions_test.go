package identity

import (
	"context"
	"encoding/base32"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/platform/totp"
)

func (r *rig) wireSessions(t *testing.T, d Deps) {
	t.Helper()
	var err error
	r.mfa, err = NewMFA(MFADeps{
		Repo: r.repo, Users: r.repo, Box: testBox(t), Auditor: d.Auditor,
		Clock: r.clock, Issuer: "ZellaPay",
	})
	if err != nil {
		t.Fatal(err)
	}
	r.sessions, err = NewSessions(SessionDeps{
		Repo: r.repo, Users: r.repo, Accounts: r.svc, Factor: r.mfa,
		Throttle: r.throttle, Clock: r.clock, Log: d.Log,
	}, MerchantSessionConfig())
	if err != nil {
		t.Fatal(err)
	}
}

func (r *rig) login(email, password string) (Issued, error) {
	return r.sessions.Login(context.Background(), LoginInput{
		Email: email, Password: password, Source: Source{IP: "198.51.100.7", UserAgent: "test"},
	})
}

// secretOf recovers the authenticator secret from the manual key.
func secretOf(t *testing.T, manualKey string) []byte {
	t.Helper()
	b, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(manualKey)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func (r *rig) codeAt(secret []byte, after time.Duration) string {
	return totp.Code(secret, totp.Step(r.clock.now.Add(after)))
}

// enrolled signs a new user up, signs in and sets up MFA. It returns the
// secret, the recovery codes and the session that completed enrolment.
func (r *rig) enrolled(t *testing.T) (secret []byte, codes []string, s Issued) {
	t.Helper()
	r.signUp(t, "jane@example.com")
	s, err := r.login("jane@example.com", goodPhrase)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	en, err := r.sessions.BeginMFAEnrolment(ctx, s.Token)
	if err != nil {
		t.Fatal(err)
	}
	secret = secretOf(t, en.ManualKey)
	codes, err = r.sessions.ConfirmMFAEnrolment(ctx, MFAInput{
		Token: s.Token, Code: r.codeAt(secret, 0), IP: "198.51.100.7",
	})
	if err != nil {
		t.Fatal(err)
	}
	return secret, codes, s
}
