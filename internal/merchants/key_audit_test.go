package merchants

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/audit"
)

var actor = &audit.Actor{Kind: audit.ActorMerchantUser, ID: "usr_1"}

func auditedEnv(t *testing.T) env {
	t.Helper()
	e := newEnv(t, ModeTest)
	e.svc.WithAuditor(audit.NewService(nil, e.clock))
	return e
}

func (e env) spec(by *audit.Actor) KeySpec {
	return KeySpec{
		MerchantID: e.mer.ID, Mode: ModeTest, Scopes: []Scope{ScopeRead}, Label: "Shop", By: by,
	}
}

func TestKeyChangesByAPersonAreAudited(t *testing.T) {
	e := auditedEnv(t)
	ctx := context.Background()
	issued, err := e.svc.IssueKey(ctx, e.spec(actor))
	if err != nil {
		t.Fatal(err)
	}
	ref := KeyRef{MerchantID: e.mer.ID, KeyID: issued.Key.ID, By: actor}
	rotated, err := e.svc.RotateKey(ctx, ref, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.Key.Label != "Shop" {
		t.Fatalf("a rotated key lost its label: %q", rotated.Key.Label)
	}
	ref.KeyID = rotated.Key.ID
	if err := e.svc.RevokeKeyRef(ctx, ref); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, a := range e.repo.audits {
		got = append(got, a.Action)
	}
	want := []string{"api_key.created", "api_key.rotated", "api_key.revoked"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("audit actions = %v, want %v", got, want)
	}
}

func TestKeyChangesWithNoActorAreNotAudited(t *testing.T) {
	e := auditedEnv(t)
	if _, err := e.svc.IssueKey(context.Background(), e.spec(nil)); err != nil {
		t.Fatal(err)
	}
	if len(e.repo.audits) != 0 {
		t.Fatal("a command-line change was audited as if a person made it")
	}
}

func TestANamedActorWithoutAnAuditorIsRefused(t *testing.T) {
	e := newEnv(t, ModeTest)
	_, err := e.svc.IssueKey(context.Background(), e.spec(actor))
	if err == nil || !errors.Is(err, errNoAuditor) {
		t.Fatalf("err = %v, want the missing-auditor error", err)
	}
	if len(e.repo.keys) != 0 {
		t.Fatal("a key was stored even though it could not be audited")
	}
}

func TestAKeyLabelIsBounded(t *testing.T) {
	e := auditedEnv(t)
	spec := e.spec(nil)
	spec.Label = string(make([]byte, 65))
	if _, err := e.svc.IssueKey(context.Background(), spec); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v", err)
	}
}
