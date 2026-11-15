package identity

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/audit"
)

const goodPhrase = "tournesol bleu sur la table"

var system = audit.Actor{Kind: audit.ActorSystem, ID: "bootstrap"}

func tokenIn(t *testing.T, link string) string {
	t.Helper()
	_, token, ok := strings.Cut(link, "?token=")
	if !ok {
		t.Fatalf("no token in %q", link)
	}
	return token
}

func (r *rig) invite(t *testing.T, email string) string {
	t.Helper()
	_, err := r.svc.Invite(context.Background(), InviteInput{
		MerchantID: "mer_1", Email: email, Role: RoleOwner, InvitedBy: system,
	})
	if err != nil {
		t.Fatal(err)
	}
	return tokenIn(t, r.mail.last(t).Data["link"])
}

func (r *rig) signUp(t *testing.T, email string) User {
	t.Helper()
	u, err := r.svc.AcceptInvitation(context.Background(),
		AcceptInput{Token: r.invite(t, email), Name: "Jane Doe", Password: goodPhrase})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestInvite_SendsOneLinkAndAuditsIt(t *testing.T) {
	r := newRig(t)
	token := r.invite(t, "  Jane@Example.com ")
	m := r.mail.last(t)
	if m.To != "jane@example.com" || m.Template != "invitation" {
		t.Fatalf("unexpected message %+v", m)
	}
	if !strings.HasPrefix(m.Data["link"], "https://merchant.test/accept-invitation?token=") {
		t.Fatalf("bad link %q", m.Data["link"])
	}
	if len(token) < 43 || m.Data["days"] != "7" || m.Data["business"] != "Boutique Soleil" {
		t.Fatalf("token %q days %q", token, m.Data["days"])
	}
	if len(r.repo.audits) != 1 || r.repo.audits[0].Action != "invitation.created" {
		t.Fatalf("audit = %+v", r.repo.audits)
	}
}

func TestInvite_RejectsBadInput(t *testing.T) {
	r := newRig(t)
	bad := []InviteInput{
		{MerchantID: "mer_1", Email: "nope", Role: RoleOwner, InvitedBy: system},
		{MerchantID: "mer_1", Email: "a@b.ga", Role: "king", InvitedBy: system},
		{MerchantID: "mer_1", Email: "a@b.ga", Role: RoleOwner, Locale: "de", InvitedBy: system},
		{MerchantID: "", Email: "a@b.ga", Role: RoleOwner, InvitedBy: system},
	}
	for _, in := range bad {
		if _, err := r.svc.Invite(context.Background(), in); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%+v: err = %v", in, err)
		}
	}
	_, err := r.svc.Invite(context.Background(), InviteInput{
		MerchantID: "mer_x", Email: "a@b.ga", Role: RoleOwner, InvitedBy: system,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown merchant: err = %v", err)
	}
}

func TestAcceptInvitation_CreatesAVerifiedUserOnce(t *testing.T) {
	r := newRig(t)
	token := r.invite(t, "jane@example.com")
	in := AcceptInput{Token: token, Name: " Jane Doe ", Password: goodPhrase}
	u, err := r.svc.AcceptInvitation(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if u.Name != "Jane Doe" || u.Role != RoleOwner || u.Status != StatusActive {
		t.Fatalf("user = %+v", u)
	}
	if u.EmailVerifiedAt == nil {
		t.Fatal("following an invitation should verify the email")
	}
	if strings.Contains(u.PasswordHash, "tournesol") {
		t.Fatal("password stored in clear")
	}
	_, err = r.svc.AcceptInvitation(context.Background(), in)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("second use: err = %v", err)
	}
}

func (r *rig) accept(token, name, password string) error {
	_, err := r.svc.AcceptInvitation(context.Background(),
		AcceptInput{Token: token, Name: name, Password: password})
	return err
}

func TestAcceptInvitation_RefusesBadTokensAndPasswords(t *testing.T) {
	r := newRig(t)
	token := r.invite(t, "jane@example.com")
	if err := r.accept("wrong", "J", goodPhrase); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("unknown token: %v", err)
	}
	if err := r.accept(token, "J", "short"); !errors.Is(err, ErrWeakPassword) {
		t.Errorf("weak password: %v", err)
	}
	if err := r.accept(token, " ", goodPhrase); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("blank name: %v", err)
	}
	r.clock.advance(8 * 24 * time.Hour)
	if err := r.accept(token, "J", goodPhrase); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expired: %v", err)
	}
}

func TestInvite_AgainReplacesTheEarlierLink(t *testing.T) {
	r := newRig(t)
	first := r.invite(t, "jane@example.com")
	second := r.invite(t, "jane@example.com")
	in := AcceptInput{Token: first, Name: "J", Password: goodPhrase}
	_, err := r.svc.AcceptInvitation(context.Background(), in)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("old link: err = %v", err)
	}
	in.Token = second
	if _, err := r.svc.AcceptInvitation(context.Background(), in); err != nil {
		t.Fatalf("new link: %v", err)
	}
}

func TestAcceptInvitation_RefusesAnAddressThatAlreadyHasAnAccount(t *testing.T) {
	r := newRig(t)
	r.signUp(t, "jane@example.com")
	in := AcceptInput{Token: r.invite(t, "jane@example.com"), Name: "J", Password: goodPhrase}
	if _, err := r.svc.AcceptInvitation(context.Background(), in); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("err = %v", err)
	}
}
