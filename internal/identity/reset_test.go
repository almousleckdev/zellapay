package identity

import (
	"context"
	"errors"
	"testing"
	"time"
)

const next = "un autre mot de passe"

func TestPasswordReset_ChangesThePasswordOnce(t *testing.T) {
	r := newRig(t)
	u := r.signUp(t, "jane@example.com")
	ctx := context.Background()
	if err := r.svc.RequestPasswordReset(ctx, "JANE@example.com"); err != nil {
		t.Fatal(err)
	}
	m := r.mail.last(t)
	if m.Template != "password_reset" || m.Data["minutes"] != "30" {
		t.Fatalf("message = %+v", m)
	}
	in := ResetInput{Token: tokenIn(t, m.Data["link"]), NewPassword: next}
	if err := r.svc.CompletePasswordReset(ctx, in); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.Authenticate(ctx, u.Email, goodPhrase); err == nil {
		t.Error("old password still works")
	}
	if _, err := r.svc.Authenticate(ctx, u.Email, next); err != nil {
		t.Errorf("new password: %v", err)
	}
	if err := r.svc.CompletePasswordReset(ctx, in); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("link reused: %v", err)
	}
}

func TestPasswordReset_SaysNothingAboutWhoHasAnAccount(t *testing.T) {
	r := newRig(t)
	r.signUp(t, "jane@example.com")
	before := len(r.mail.sent)
	if err := r.svc.RequestPasswordReset(context.Background(), "nobody@example.com"); err != nil {
		t.Fatalf("unknown address: err = %v", err)
	}
	if len(r.mail.sent) != before {
		t.Fatal("an email was sent for an unknown address")
	}
}

func TestPasswordReset_NewRequestReplacesTheOldLink(t *testing.T) {
	r := newRig(t)
	r.signUp(t, "jane@example.com")
	ctx := context.Background()
	_ = r.svc.RequestPasswordReset(ctx, "jane@example.com")
	old := tokenIn(t, r.mail.last(t).Data["link"])
	_ = r.svc.RequestPasswordReset(ctx, "jane@example.com")
	err := r.svc.CompletePasswordReset(ctx, ResetInput{Token: old, NewPassword: next})
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("old link: err = %v", err)
	}
}

func TestPasswordReset_ExpiresAndEnforcesThePolicy(t *testing.T) {
	r := newRig(t)
	r.signUp(t, "jane@example.com")
	ctx := context.Background()
	_ = r.svc.RequestPasswordReset(ctx, "jane@example.com")
	token := tokenIn(t, r.mail.last(t).Data["link"])
	err := r.svc.CompletePasswordReset(ctx, ResetInput{Token: token, NewPassword: "short"})
	if !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("weak: %v", err)
	}
	r.clock.advance(31 * time.Minute)
	err = r.svc.CompletePasswordReset(ctx, ResetInput{Token: token, NewPassword: next})
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired: %v", err)
	}
}

func TestEmailVerification(t *testing.T) {
	r := newRig(t)
	u := r.signUp(t, "jane@example.com")
	ctx := context.Background()
	u.EmailVerifiedAt = nil
	r.repo.users[u.ID] = u
	if err := r.svc.RequestEmailVerification(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	m := r.mail.last(t)
	if m.Template != "email_verification" || m.Data["hours"] != "24" {
		t.Fatalf("message = %+v", m)
	}
	token := tokenIn(t, m.Data["link"])
	if err := r.svc.ConfirmEmail(ctx, token); err != nil {
		t.Fatal(err)
	}
	if r.repo.users[u.ID].EmailVerifiedAt == nil {
		t.Fatal("email not marked verified")
	}
	if err := r.svc.ConfirmEmail(ctx, token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("reuse: %v", err)
	}
	n := len(r.mail.sent)
	_ = r.svc.RequestEmailVerification(ctx, u.ID)
	if len(r.mail.sent) != n {
		t.Fatal("verified user was sent another link")
	}
}
