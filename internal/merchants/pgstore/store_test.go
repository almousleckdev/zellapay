package pgstore_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/merchants/pgstore"
	"github.com/zellapay/zellapay/internal/platform/clock"
	"github.com/zellapay/zellapay/internal/platform/pgtest"
)

func newService(t *testing.T) (*merchants.Service, merchants.Merchant) {
	t.Helper()
	repo := pgstore.New(pgtest.NewPool(t, testDSN))
	svc := merchants.NewService(repo, clock.System{}, merchants.ModeTest)
	m, err := svc.CreateMerchant(context.Background(), "Acme", "XAF")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetStatus(context.Background(), m.ID, merchants.StatusActive); err != nil {
		t.Fatal(err)
	}
	return svc, m
}

func issue(t *testing.T, svc *merchants.Service, m merchants.Merchant) merchants.IssuedKey {
	t.Helper()
	k, err := svc.IssueKey(context.Background(), merchants.KeySpec{
		MerchantID:  m.ID,
		Mode:        merchants.ModeTest,
		Scopes:      []merchants.Scope{merchants.ScopeRead, merchants.ScopeCollectionsWrite},
		IPAllowlist: []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")},
	})
	if err != nil {
		t.Fatal(err)
	}
	return k
}

var okIP = netip.MustParseAddr("203.0.113.9")

func TestIssueAndAuthenticate(t *testing.T) {
	svc, m := newService(t)
	k := issue(t, svc, m)

	p, err := svc.Authenticate(context.Background(), k.Secret, okIP)
	if err != nil {
		t.Fatal(err)
	}
	if p.MerchantID != m.ID || !p.Has(merchants.ScopeCollectionsWrite) {
		t.Fatalf("unexpected principal %+v", p)
	}
	_, err = svc.Authenticate(context.Background(), k.Secret, netip.MustParseAddr("192.0.2.1"))
	if !errors.Is(err, merchants.ErrIPNotAllowed) {
		t.Fatalf("got %v", err)
	}
}

func TestRevokedKeyIsRejected(t *testing.T) {
	svc, m := newService(t)
	k := issue(t, svc, m)
	if err := svc.RevokeKey(context.Background(), m.ID, k.Key.ID); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Authenticate(context.Background(), k.Secret, okIP)
	if !errors.Is(err, merchants.ErrInvalidKey) {
		t.Fatalf("got %v", err)
	}
	err = svc.RevokeKey(context.Background(), m.ID, k.Key.ID)
	if !errors.Is(err, merchants.ErrNotFound) {
		t.Fatalf("second revoke: got %v", err)
	}
}

func TestRevokeRequiresOwningMerchant(t *testing.T) {
	svc, m := newService(t)
	k := issue(t, svc, m)
	err := svc.RevokeKey(context.Background(), "mer_someoneelse", k.Key.ID)
	if !errors.Is(err, merchants.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
	if _, err := svc.Authenticate(context.Background(), k.Secret, okIP); err != nil {
		t.Fatalf("key must still work: %v", err)
	}
}

func TestRotateKeepsBothKeysDuringOverlap(t *testing.T) {
	svc, m := newService(t)
	old := issue(t, svc, m)
	ref := merchants.KeyRef{MerchantID: m.ID, KeyID: old.Key.ID}
	next, err := svc.RotateKey(context.Background(), ref, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []merchants.IssuedKey{old, next} {
		if _, err := svc.Authenticate(context.Background(), k.Secret, okIP); err != nil {
			t.Fatalf("key must work during overlap: %v", err)
		}
	}
}

func TestSuspendedMerchantCannotAuthenticate(t *testing.T) {
	svc, m := newService(t)
	k := issue(t, svc, m)
	if err := svc.SetStatus(context.Background(), m.ID, merchants.StatusSuspended); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Authenticate(context.Background(), k.Secret, okIP)
	if !errors.Is(err, merchants.ErrMerchantInactive) {
		t.Fatalf("got %v", err)
	}
}

func TestSetStatusUnknownMerchant(t *testing.T) {
	svc, _ := newService(t)
	err := svc.SetStatus(context.Background(), "mer_missing", merchants.StatusActive)
	if !errors.Is(err, merchants.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestKeysCannotBeDeleted(t *testing.T) {
	svc, m := newService(t)
	k := issue(t, svc, m)
	pool := pgtest.NewPool(t, testDSN)
	_, err := pool.Exec(context.Background(),
		"DELETE FROM merchant_api_keys WHERE id = $1", k.Key.ID)
	if err == nil {
		t.Fatal("deleting an api key must be rejected by the database")
	}
}
