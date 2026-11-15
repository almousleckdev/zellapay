package identity

import (
	"context"
	"errors"
	"testing"
	"time"
)

const time30s = 30 * time.Second

const changedPhrase = "une phrase de passe différente"

func (r *rig) change(token, current, next string) error {
	return r.sessions.ChangePassword(context.Background(), PasswordChangeInput{
		Token: token, Current: current, New: next, IP: "198.51.100.7",
	})
}

func TestChangePasswordNeedsTheCurrentOneAndEndsOtherSessions(t *testing.T) {
	r := newRig(t)
	_, _, mine := r.enrolled(t)
	other, _ := r.login("jane@example.com", goodPhrase)
	err := r.change(mine.Token, "not the current one", changedPhrase)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong current password: %v", err)
	}
	if err = r.change(mine.Token, goodPhrase, "short"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("weak new password: %v", err)
	}
	if err = r.change(mine.Token, goodPhrase, changedPhrase); err != nil {
		t.Fatal(err)
	}
	if resolve(r, mine.Token) != nil || resolve(r, other.Token) == nil {
		t.Fatal("only the session that changed the password should survive")
	}
	if _, err := r.login("jane@example.com", goodPhrase); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("the old password still works: %v", err)
	}
	if _, err := r.login("jane@example.com", changedPhrase); err != nil {
		t.Fatalf("the new password: %v", err)
	}
	if last := r.repo.audits[len(r.repo.audits)-1]; last.Action != "password.changed" {
		t.Fatalf("last audit entry = %q", last.Action)
	}
}

func TestGuessingTheCurrentPasswordIsSlowedDown(t *testing.T) {
	r := newRig(t)
	_, _, s := r.enrolled(t)
	for range 3 {
		_ = r.change(s.Token, "wrong wrong wrong", changedPhrase)
	}
	if err := r.change(s.Token, goodPhrase, changedPhrase); !errors.Is(err, ErrThrottled) {
		t.Fatalf("the right password got past the lockout: %v", err)
	}
}

func TestNewRecoveryCodesReplaceTheOldOnes(t *testing.T) {
	r := newRig(t)
	_, old, s := r.enrolled(t)
	ctx := context.Background()
	fresh, err := r.sessions.RegenerateRecoveryCodes(ctx, s.Token)
	if err != nil || len(fresh) != 10 {
		t.Fatalf("fresh = %d codes, %v", len(fresh), err)
	}
	next, _ := r.login("jane@example.com", goodPhrase)
	in := MFAInput{Token: next.Token, Code: old[0], IP: "198.51.100.7"}
	if _, err := r.sessions.CompleteMFA(ctx, in); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("an old code still works: %v", err)
	}
	in.Code = fresh[0]
	if _, err := r.sessions.CompleteMFA(ctx, in); err != nil {
		t.Fatalf("a new code: %v", err)
	}
}

func TestRemovingTheAuthenticatorMeansEnrollingAgain(t *testing.T) {
	r := newRig(t)
	_, _, mine := r.enrolled(t)
	other, _ := r.login("jane@example.com", goodPhrase)
	ctx := context.Background()
	if err := r.sessions.RemoveAuthenticator(ctx, mine.Token); err != nil {
		t.Fatal(err)
	}
	if resolve(r, other.Token) == nil {
		t.Fatal("another session survived the removal")
	}
	p, err := r.sessions.Resolve(ctx, mine.Token)
	if err != nil || p.MFA != MFAEnrolRequired {
		t.Fatalf("this session: %+v, %v", p, err)
	}
	if _, err := r.sessions.BeginMFAEnrolment(ctx, mine.Token); err != nil {
		t.Fatalf("enrolling again: %v", err)
	}
}

func TestAnAnsweredSessionCanProveTheSecondFactorAgain(t *testing.T) {
	r := newRig(t)
	secret, _, s := r.enrolled(t)
	r.clock.advance(time30s)
	in := MFAInput{Token: s.Token, Code: r.codeAt(secret, 0), IP: "198.51.100.7"}
	if _, err := r.sessions.CompleteMFA(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	p, _ := r.sessions.Resolve(context.Background(), s.Token)
	if !p.Session.MFAFresh(r.clock.now, time30s) {
		t.Fatal("the second answer did not refresh the proof")
	}
}
