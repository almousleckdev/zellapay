package pgstore_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/zellapay/zellapay/internal/identity"
)

func TestInvitationToSignIn(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	inv := w.invite(t, "jane@example.com")
	token := w.tokenFor(t, "invitations", inv.ID)

	u, err := w.svc.AcceptInvitation(ctx,
		identity.AcceptInput{Token: token, Name: "Jane", Password: phrase})
	if err != nil {
		t.Fatal(err)
	}
	got, err := w.svc.Authenticate(ctx, "jane@example.com", phrase)
	if err != nil || got.ID != u.ID || got.MerchantID != "mer_1" || got.EmailVerifiedAt == nil {
		t.Fatalf("sign-in: %+v, %v", got, err)
	}
	if n := w.count(t, "SELECT count(*) FROM notifications WHERE template = 'invitation'"); n != 1 {
		t.Errorf("invitation emails queued = %d, want 1", n)
	}
	if n := w.count(t, "SELECT count(*) FROM audit_log"); n != 2 {
		t.Errorf("audit entries = %d, want invitation.created and user.created", n)
	}
}

func TestNoSecretIsStoredInTheClear(t *testing.T) {
	w := newWorld(t)
	inv := w.invite(t, "jane@example.com")
	token := w.tokenFor(t, "invitations", inv.ID)
	if _, err := w.svc.AcceptInvitation(context.Background(),
		identity.AcceptInput{Token: token, Name: "Jane", Password: phrase}); err != nil {
		t.Fatal(err)
	}
	var hash string
	if err := w.pool.QueryRow(context.Background(),
		"SELECT password_hash FROM merchant_users").Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") || strings.Contains(hash, "tournesol") {
		t.Fatalf("phrase hash = %q", hash)
	}
	if n := w.count(t, "SELECT count(*) FROM audit_log WHERE change::text LIKE '%jane@%'"); n != 0 {
		t.Fatal("an email address reached the audit log")
	}
}

func TestOnlyOneRequestCanUseALink(t *testing.T) {
	w := newWorld(t)
	inv := w.invite(t, "jane@example.com")
	token := w.tokenFor(t, "invitations", inv.ID)
	var wins, losses atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := w.svc.AcceptInvitation(context.Background(),
				identity.AcceptInput{Token: token, Name: "Jane", Password: phrase})
			switch {
			case err == nil:
				wins.Add(1)
			case errors.Is(err, identity.ErrInvalidToken), errors.Is(err, identity.ErrEmailTaken):
				losses.Add(1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 || losses.Load() != 7 {
		t.Fatalf("wins=%d losses=%d, want 1 and 7", wins.Load(), losses.Load())
	}
	if n := w.count(t, "SELECT count(*) FROM merchant_users"); n != 1 {
		t.Fatalf("users = %d", n)
	}
}

func TestAFailedChangeLeavesNoEmailAndNoAuditEntry(t *testing.T) {
	w := newWorld(t)
	first := w.invite(t, "jane@example.com")
	u, err := w.svc.AcceptInvitation(context.Background(), identity.AcceptInput{
		Token: w.tokenFor(t, "invitations", first.ID), Name: "Jane", Password: phrase})
	if err != nil {
		t.Fatal(err)
	}
	audits := w.count(t, "SELECT count(*) FROM audit_log")
	w.exec(t, "DELETE FROM merchant_users WHERE id = $1", u.ID)
	w.exec(t, `INSERT INTO merchant_users (id, merchant_id, email, name, role, status,
		locale, password_hash, password_changed_at, created_at)
		VALUES ('usr_other', 'mer_1', 'jane@example.com', 'X', 'owner', 'active',
		'fr', 'h', now(), now())`)
	second := w.invite(t, "jane@example.com")
	audits++ // the new invitation.created
	_, err = w.svc.AcceptInvitation(context.Background(), identity.AcceptInput{
		Token: w.tokenFor(t, "invitations", second.ID), Name: "Jane", Password: phrase})
	if !errors.Is(err, identity.ErrEmailTaken) {
		t.Fatalf("err = %v, want ErrEmailTaken", err)
	}
	if got := w.count(t, "SELECT count(*) FROM audit_log"); got != audits {
		t.Fatalf("audit entries = %d, want %d: a failed change was audited", got, audits)
	}
	if n := w.count(t, "SELECT count(*) FROM invitations WHERE used_at IS NOT NULL"); n != 1 {
		t.Fatalf("used invitations = %d: the failed accept consumed its link", n)
	}
}

func TestPasswordResetAndVerificationOnRealStorage(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	inv := w.invite(t, "jane@example.com")
	u, err := w.svc.AcceptInvitation(ctx, identity.AcceptInput{
		Token: w.tokenFor(t, "invitations", inv.ID), Name: "Jane", Password: phrase})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.svc.RequestPasswordReset(ctx, "jane@example.com"); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := w.pool.QueryRow(ctx, "SELECT id FROM email_tokens").Scan(&id); err != nil {
		t.Fatal(err)
	}
	token := w.tokenFor(t, "email_tokens", id)
	next := "un autre mot de passe"
	reset := identity.ResetInput{Token: token, NewPassword: next}
	if err := w.svc.CompletePasswordReset(ctx, reset); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.Authenticate(ctx, u.Email, next); err != nil {
		t.Fatalf("new password: %v", err)
	}
	err = w.svc.CompletePasswordReset(ctx, reset)
	if !errors.Is(err, identity.ErrInvalidToken) {
		t.Fatalf("link reused: %v", err)
	}
}
