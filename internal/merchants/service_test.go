package merchants

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"
)

var testNow = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

type env struct {
	svc   *Service
	clock *fakeClock
	repo  *fakeRepo
	mer   Merchant
}

func newEnv(t *testing.T, mode Mode) env {
	t.Helper()
	clk := &fakeClock{now: testNow}
	repo := newFakeRepo()
	svc := NewService(repo, clk, mode)
	m, err := svc.CreateMerchant(context.Background(), "Acme", "XAF")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetStatus(context.Background(), m.ID, StatusActive); err != nil {
		t.Fatal(err)
	}
	return env{svc: svc, clock: clk, repo: repo, mer: m}
}

func (e env) issue(t *testing.T, mut func(*KeySpec)) IssuedKey {
	t.Helper()
	spec := KeySpec{MerchantID: e.mer.ID, Mode: ModeTest, Scopes: []Scope{ScopeRead}}
	if mut != nil {
		mut(&spec)
	}
	k, err := e.svc.IssueKey(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

var anyIP = netip.MustParseAddr("203.0.113.7")

func (e env) auth(raw string) (Principal, error) {
	return e.svc.Authenticate(context.Background(), raw, anyIP)
}

func TestAuthenticateAcceptsValidKey(t *testing.T) {
	e := newEnv(t, ModeTest)
	k := e.issue(t, nil)
	p, err := e.auth(k.Secret)
	if err != nil {
		t.Fatal(err)
	}
	if p.MerchantID != e.mer.ID || !p.Has(ScopeRead) || p.Has(ScopePayoutsWrite) {
		t.Fatalf("unexpected principal %+v", p)
	}
}

func TestAuthenticateAllowsListedIP(t *testing.T) {
	e := newEnv(t, ModeTest)
	k := e.issue(t, func(s *KeySpec) {
		s.IPAllowlist = []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}
	})
	if _, err := e.auth(k.Secret); err != nil {
		t.Fatal(err)
	}
}

func TestRotateKeepsOldKeyUntilOverlapEnds(t *testing.T) {
	e := newEnv(t, ModeTest)
	old := e.issue(t, nil)
	ref := KeyRef{MerchantID: e.mer.ID, KeyID: old.Key.ID}
	next, err := e.svc.RotateKey(context.Background(), ref, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []IssuedKey{old, next} {
		if _, err := e.auth(k.Secret); err != nil {
			t.Fatalf("both keys must work during overlap: %v", err)
		}
	}
	e.clock.now = testNow.Add(time.Hour)
	if _, err := e.auth(old.Secret); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("old key must expire, got %v", err)
	}
	if _, err := e.auth(next.Secret); err != nil {
		t.Fatalf("new key must keep working: %v", err)
	}
}

func TestIssueKeyValidatesSpec(t *testing.T) {
	e := newEnv(t, ModeTest)
	bad := []KeySpec{
		{MerchantID: "", Mode: ModeTest, Scopes: []Scope{ScopeRead}},
		{MerchantID: e.mer.ID, Mode: "prod", Scopes: []Scope{ScopeRead}},
		{MerchantID: e.mer.ID, Mode: ModeTest},
		{MerchantID: e.mer.ID, Mode: ModeTest, Scopes: []Scope{"admin"}},
	}
	for i, spec := range bad {
		if _, err := e.svc.IssueKey(context.Background(), spec); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("case %d: got %v", i, err)
		}
	}
}

func TestCreateMerchantValidates(t *testing.T) {
	e := newEnv(t, ModeTest)
	for _, c := range [][2]string{{"", "XAF"}, {"A", "xaf"}, {"A", "XA"}} {
		_, err := e.svc.CreateMerchant(context.Background(), c[0], c[1])
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%v: got %v", c, err)
		}
	}
}

func TestStoredKeyHoldsOnlyHash(t *testing.T) {
	e := newEnv(t, ModeTest)
	k := e.issue(t, nil)
	if string(k.Key.SecretHash) == k.Secret || len(k.Key.SecretHash) != 32 {
		t.Fatal("stored key must hold a 32-byte hash, not the secret")
	}
}

func TestFeeBPSDefaultsToZeroAndIsValidated(t *testing.T) {
	e := newEnv(t, ModeTest)
	ctx := context.Background()
	if bps, err := e.svc.FeeBPS(ctx, e.mer.ID); err != nil || bps != 0 {
		t.Fatalf("default fee = %d, %v", bps, err)
	}
	if err := e.svc.SetFeeBPS(ctx, e.mer.ID, 150); err != nil {
		t.Fatal(err)
	}
	if bps, _ := e.svc.FeeBPS(ctx, e.mer.ID); bps != 150 {
		t.Fatalf("fee = %d, want 150", bps)
	}
	for _, bad := range []int{-1, 10_001} {
		if err := e.svc.SetFeeBPS(ctx, e.mer.ID, bad); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%d: got %v", bad, err)
		}
	}
}

func TestPayoutFeeIsSeparateFromTheCollectionFee(t *testing.T) {
	e := newEnv(t, ModeTest)
	ctx := context.Background()
	if err := e.svc.SetFeeBPS(ctx, e.mer.ID, 150); err != nil {
		t.Fatal(err)
	}
	if bps, _ := e.svc.PayoutFeeBPS(ctx, e.mer.ID); bps != 0 {
		t.Fatalf("payout fee must default to 0, got %d", bps)
	}
	if err := e.svc.SetPayoutFeeBPS(ctx, e.mer.ID, 100); err != nil {
		t.Fatal(err)
	}
	if bps, _ := e.svc.PayoutFeeBPS(ctx, e.mer.ID); bps != 100 {
		t.Fatalf("payout fee = %d", bps)
	}
	if bps, _ := e.svc.FeeBPS(ctx, e.mer.ID); bps != 150 {
		t.Fatalf("collection fee must be untouched, got %d", bps)
	}
	if err := e.svc.SetPayoutFeeBPS(ctx, e.mer.ID, 10_001); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}
