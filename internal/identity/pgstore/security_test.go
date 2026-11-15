package pgstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/zellapay/zellapay/internal/identity"
)

func TestChangingThePasswordKeepsOnlyTheCurrentSession(t *testing.T) {
	w := newWorld(t)
	w.member(t)
	mine, other := w.login(t), w.login(t)
	ctx := context.Background()
	err := w.sessions.ChangePassword(ctx, identity.PasswordChangeInput{
		Token: mine.Token, Current: phrase, New: "une phrase de passe différente", IP: src.IP,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.sessions.Resolve(ctx, mine.Token); err != nil {
		t.Fatalf("the current session ended: %v", err)
	}
	if _, err := w.sessions.Resolve(ctx, other.Token); !errors.Is(err, identity.ErrInvalidSession) {
		t.Fatalf("another session survived: %v", err)
	}
	if n := w.audited(t, "password.changed"); n != 1 {
		t.Fatalf("audit entries = %d", n)
	}
}

func TestRegeneratedRecoveryCodesReplaceTheStoredOnes(t *testing.T) {
	w := newWorld(t)
	w.member(t)
	_, old := w.enrol(t)
	s := w.login(t)
	fresh, err := w.sessions.RegenerateRecoveryCodes(context.Background(), s.Token)
	if err != nil || len(fresh) != 10 {
		t.Fatalf("fresh = %d, %v", len(fresh), err)
	}
	if n := w.count(t, "SELECT count(*) FROM recovery_codes"); n != 10 {
		t.Fatalf("stored codes = %d, want exactly the new 10", n)
	}
	in := identity.MFAInput{Token: s.Token, Code: old[0], IP: src.IP}
	_, err = w.sessions.CompleteMFA(context.Background(), in)
	if !errors.Is(err, identity.ErrInvalidCode) {
		t.Fatalf("an old code still works: %v", err)
	}
}

func TestRemovingTheAuthenticatorLeavesNothingBehind(t *testing.T) {
	w := newWorld(t)
	w.member(t)
	w.enrol(t)
	mine, other := w.login(t), w.login(t)
	ctx := context.Background()
	if _, err := w.sessions.CompleteMFA(ctx, identity.MFAInput{
		Token: mine.Token, Code: w.recovery(t), IP: src.IP,
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.sessions.RemoveAuthenticator(ctx, mine.Token); err != nil {
		t.Fatal(err)
	}
	if n := w.count(t, "SELECT count(*) FROM mfa_credentials"); n != 0 {
		t.Fatalf("credentials left = %d", n)
	}
	if n := w.count(t, "SELECT count(*) FROM recovery_codes"); n != 0 {
		t.Fatalf("recovery codes left = %d", n)
	}
	if _, err := w.sessions.Resolve(ctx, other.Token); !errors.Is(err, identity.ErrInvalidSession) {
		t.Fatalf("another session survived: %v", err)
	}
	p, err := w.sessions.Resolve(ctx, mine.Token)
	if err != nil || p.MFA != identity.MFAEnrolRequired {
		t.Fatalf("this session: %+v, %v", p, err)
	}
}
