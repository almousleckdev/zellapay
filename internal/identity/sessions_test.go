package identity

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestFirstSignInMustEnrolMFA(t *testing.T) {
	r := newRig(t)
	r.signUp(t, "jane@example.com")
	s, err := r.login("Jane@Example.com", goodPhrase)
	if err != nil || s.MFA != MFAEnrolRequired || len(s.Token) < 43 {
		t.Fatalf("issued = %+v, %v", s, err)
	}
	p, err := r.sessions.Resolve(context.Background(), s.Token)
	if err != nil || p.User.Email != "jane@example.com" || p.MFA != MFAEnrolRequired {
		t.Fatalf("principal = %+v, %v", p, err)
	}
}

func TestEnrolmentGivesTenRecoveryCodesAndAFreshSession(t *testing.T) {
	r := newRig(t)
	_, codes, s := r.enrolled(t)
	if len(codes) != 10 || len(codes[0]) != 19 || strings.Count(codes[0], "-") != 3 {
		t.Fatalf("codes = %v", codes)
	}
	p, err := r.sessions.Resolve(context.Background(), s.Token)
	if err != nil || p.MFA != MFADone || !p.Session.MFAFresh(r.clock.now, 10*time.Minute) {
		t.Fatalf("principal = %+v, %v", p, err)
	}
	if r.repo.audits[len(r.repo.audits)-1].Action != "mfa.enrolled" {
		t.Fatal("enrolment was not audited")
	}
	_, err = r.sessions.BeginMFAEnrolment(context.Background(), s.Token)
	if !errors.Is(err, ErrMFAAlreadyEnrolled) {
		t.Fatalf("second enrolment: %v", err)
	}
}

func TestEnrolmentNeedsTheRightCode(t *testing.T) {
	r := newRig(t)
	r.signUp(t, "jane@example.com")
	s, _ := r.login("jane@example.com", goodPhrase)
	ctx := context.Background()
	if _, err := r.sessions.BeginMFAEnrolment(ctx, s.Token); err != nil {
		t.Fatal(err)
	}
	_, err := r.sessions.ConfirmMFAEnrolment(ctx, MFAInput{Token: s.Token, Code: "000000"})
	if !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("err = %v", err)
	}
	if p, _ := r.sessions.Resolve(ctx, s.Token); p.MFA != MFAEnrolRequired {
		t.Fatal("a wrong code activated MFA")
	}
}

func TestNextSignInAsksForACodeAndRefusesAReplay(t *testing.T) {
	r := newRig(t)
	secret, _, _ := r.enrolled(t)
	r.clock.advance(time.Minute)
	s, err := r.login("jane@example.com", goodPhrase)
	if err != nil || s.MFA != MFARequired {
		t.Fatalf("issued = %+v, %v", s, err)
	}
	ctx := context.Background()
	in := MFAInput{Token: s.Token, Code: "000000", IP: "198.51.100.7"}
	if _, err := r.sessions.CompleteMFA(ctx, in); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("wrong code: %v", err)
	}
	in.Code = r.codeAt(secret, 0)
	if _, err := r.sessions.CompleteMFA(ctx, in); err != nil {
		t.Fatalf("right code: %v", err)
	}
	other, _ := r.login("jane@example.com", goodPhrase)
	in.Token = other.Token
	if _, err := r.sessions.CompleteMFA(ctx, in); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("the same code worked twice: %v", err)
	}
}

func TestARecoveryCodeWorksOnceAndSaysSo(t *testing.T) {
	r := newRig(t)
	_, codes, _ := r.enrolled(t)
	ctx := context.Background()
	s, _ := r.login("jane@example.com", goodPhrase)
	in := MFAInput{Token: s.Token, Code: strings.ToLower(codes[0]), IP: "198.51.100.7"}
	v, err := r.sessions.CompleteMFA(ctx, in)
	if err != nil || !v.RecoveryUsed || v.CodesLeft != 9 {
		t.Fatalf("v = %+v, err = %v", v, err)
	}
	again, _ := r.login("jane@example.com", goodPhrase)
	in.Token = again.Token
	if _, err := r.sessions.CompleteMFA(ctx, in); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("code reused: %v", err)
	}
	if last := r.repo.audits[len(r.repo.audits)-1]; last.Action != "mfa.recovery_code_used" {
		t.Fatalf("recovery use was not audited, last entry %q", last.Action)
	}
}

func TestGuessingIsSlowedBeforeThePasswordIsChecked(t *testing.T) {
	r := newRig(t)
	r.signUp(t, "jane@example.com")
	for range 3 {
		_, err := r.login("jane@example.com", "wrong wrong wrong")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("err = %v", err)
		}
	}
	_, err := r.login("jane@example.com", goodPhrase)
	var te *ThrottledError
	if !errors.Is(err, ErrThrottled) || !errors.As(err, &te) || te.RetryAfter != time.Minute {
		t.Fatalf("the right password got through a lockout: %v", err)
	}
	if _, err := r.login("ghost@example.com", "x"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("an unknown account must be treated alike: %v", err)
	}
}

func TestAnUnreachableThrottleBlocksSignIn(t *testing.T) {
	r := newRig(t)
	r.signUp(t, "jane@example.com")
	r.throttle.down = true
	if _, err := r.login("jane@example.com", goodPhrase); err == nil {
		t.Fatal("sign-in worked while the throttle was down")
	}
}
