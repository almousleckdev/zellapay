package pgstore_test

import (
	"context"
	"encoding/base32"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/totp"
)

var src = identity.Source{IP: "198.51.100.7", UserAgent: "test"}

func (w *world) member(t *testing.T) identity.User {
	t.Helper()
	inv := w.invite(t, "jane@example.com")
	u, err := w.svc.AcceptInvitation(context.Background(), identity.AcceptInput{
		Token: w.tokenFor(t, "invitations", inv.ID), Name: "Jane", Password: phrase})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func (w *world) login(t *testing.T) identity.Issued {
	t.Helper()
	s, err := w.sessions.Login(context.Background(), identity.LoginInput{
		Email: "jane@example.com", Password: phrase, Source: src,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// enrol sets MFA up and returns the secret and the recovery codes.
func (w *world) enrol(t *testing.T) ([]byte, []string) {
	t.Helper()
	ctx := context.Background()
	s := w.login(t)
	en, err := w.sessions.BeginMFAEnrolment(ctx, s.Token)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(en.ManualKey)
	if err != nil {
		t.Fatal(err)
	}
	codes, err := w.sessions.ConfirmMFAEnrolment(ctx, identity.MFAInput{
		Token: s.Token, Code: totp.Code(secret, totp.Step(w.clock.Now())), IP: src.IP,
	})
	if err != nil {
		t.Fatal(err)
	}
	return secret, codes
}

func TestSignInThroughEnrolmentToAFreshSession(t *testing.T) {
	w := newWorld(t)
	w.member(t)
	_, codes := w.enrol(t)
	if len(codes) != 10 {
		t.Fatalf("codes = %d", len(codes))
	}
	s := w.login(t)
	if s.MFA != identity.MFARequired {
		t.Fatalf("second sign-in needs a code, got %q", s.MFA)
	}
	p, err := w.sessions.Resolve(context.Background(), s.Token)
	if err != nil || p.User.Email != "jane@example.com" || p.Session.Source != src {
		t.Fatalf("principal = %+v, %v", p, err)
	}
}

func TestNoSecretReachesTheDatabaseInTheClear(t *testing.T) {
	w := newWorld(t)
	w.member(t)
	secret, codes := w.enrol(t)
	s := w.login(t)
	manual := totp.Encode(secret)
	if w.holds(t, "mfa_credentials", "secret_enc", manual) {
		t.Fatal("the authenticator secret is stored in the clear")
	}
	plain := strings.ReplaceAll(codes[0], "-", "")
	if w.holds(t, "recovery_codes", "encode(code_hash, 'escape')", plain) {
		t.Fatal("a recovery code is stored in the clear")
	}
	if w.holds(t, "sessions", "encode(token_hash, 'escape')", s.Token) {
		t.Fatal("a session token is stored in the clear")
	}
}

// holds reports whether any row's column contains needle.
func (w *world) holds(t *testing.T, table, column, needle string) bool {
	t.Helper()
	q := "SELECT count(*) FROM " + table + " WHERE " + column + " LIKE '%' || $1 || '%'"
	return w.count(t, q, needle) > 0
}

func (w *world) audited(t *testing.T, action string) int {
	t.Helper()
	return w.count(t, "SELECT count(*) FROM audit_log WHERE action = $1", action)
}

func (w *world) pendingSessions(t *testing.T, n int) []string {
	t.Helper()
	tokens := make([]string, n)
	for i := range tokens {
		tokens[i] = w.login(t).Token
	}
	return tokens
}

func TestOneCodeIsAcceptedOnceEvenWhenRaced(t *testing.T) {
	w := newWorld(t)
	w.member(t)
	secret, _ := w.enrol(t)
	w.clock.advance(30 * time.Second)
	code := totp.Code(secret, totp.Step(w.clock.Now()))
	tokens := w.pendingSessions(t, 8)
	var wins atomic.Int32
	var wg sync.WaitGroup
	for _, tok := range tokens {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := w.sessions.CompleteMFA(context.Background(),
				identity.MFAInput{Token: tok, Code: code, IP: src.IP})
			if err == nil {
				wins.Add(1)
			} else if !errors.Is(err, identity.ErrInvalidCode) {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("%d sessions accepted one code, want 1", wins.Load())
	}
}

func TestOneRecoveryCodeIsSpentOnceEvenWhenRaced(t *testing.T) {
	w := newWorld(t)
	w.member(t)
	_, codes := w.enrol(t)
	tokens := w.pendingSessions(t, 8)
	var wins atomic.Int32
	var wg sync.WaitGroup
	for _, tok := range tokens {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := w.sessions.CompleteMFA(context.Background(),
				identity.MFAInput{Token: tok, Code: codes[0], IP: src.IP})
			if err == nil {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("%d sessions spent one recovery code, want 1", wins.Load())
	}
	if n := w.audited(t, "mfa.recovery_code_used"); n != 1 {
		t.Fatalf("audit entries = %d, want 1", n)
	}
}

func TestAWrongRecoveryGuessLeavesNoAuditEntry(t *testing.T) {
	w := newWorld(t)
	w.member(t)
	w.enrol(t)
	s := w.login(t)
	_, err := w.sessions.CompleteMFA(context.Background(),
		identity.MFAInput{Token: s.Token, Code: "AAAA-BBBB-CCCC-DDDD", IP: src.IP})
	if !errors.Is(err, identity.ErrInvalidCode) {
		t.Fatalf("err = %v", err)
	}
	if n := w.audited(t, "mfa.recovery_code_used"); n != 0 {
		t.Fatalf("a failed guess was audited %d times", n)
	}
}

func TestResettingThePasswordEndsSessionsInTheDatabase(t *testing.T) {
	w := newWorld(t)
	w.member(t)
	s := w.login(t)
	ctx := context.Background()
	if err := w.svc.RequestPasswordReset(ctx, "jane@example.com"); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := w.pool.QueryRow(ctx, "SELECT id FROM email_tokens").Scan(&id); err != nil {
		t.Fatal(err)
	}
	next := "un autre mot de passe"
	err := w.svc.CompletePasswordReset(ctx, identity.ResetInput{
		Token: w.tokenFor(t, "email_tokens", id), NewPassword: next})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.sessions.Resolve(ctx, s.Token); !errors.Is(err, identity.ErrInvalidSession) {
		t.Fatalf("a session survived the reset: %v", err)
	}
}
