package pgstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/ledger"
)

func (w *world) events(t *testing.T, id string) []string {
	t.Helper()
	evs, err := w.store.Events(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	types := make([]string, len(evs))
	for i, e := range evs {
		types[i] = e.Type
	}
	return types
}

func (w *world) assertNoMoney(t *testing.T, c collections.Collection) {
	t.Helper()
	_, err := w.ledger.Balance(context.Background(), ledger.MerchantPending(c.MerchantID))
	if !errors.Is(err, ledger.ErrUnknownAccount) {
		t.Fatalf("no money may be recorded, got %v", err)
	}
}

func TestWrongAmountCallbackHoldsTheCollectionForAPerson(t *testing.T) {
	w := newWorld(t, 150)
	c := w.create(t, "07")
	w.clock.advance(simDelay + 1)
	w.deliverCallbacks(t)

	got := w.reload(t, c.ID)
	held := got.Status == collections.StatusAmbiguous &&
		got.FailureCode == collections.CodeAmountMismatch
	if !held {
		t.Fatalf("got %+v", got)
	}
	if got.NextCheckAt != nil {
		t.Fatal("a held collection must not be re-checked automatically")
	}
	w.clock.advance(48 * time.Hour)
	if res, err := w.svc.Sweep(context.Background(), 100); err != nil || res.Checked != 0 {
		t.Fatalf("sweep must leave it alone: %+v, %v", res, err)
	}
	w.assertNoMoney(t, c)
}

func TestCollectionStuckPendingIsFlaggedNotExpired(t *testing.T) {
	w := newWorld(t, 150)
	c := w.create(t, "04")
	cfg := collections.DefaultConfig()
	w.clock.advance(cfg.StaleAfter + time.Minute)
	if _, err := w.svc.Sweep(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	got := w.reload(t, c.ID)
	if got.Status != collections.StatusPending {
		t.Fatalf("only the operator may end it, got %s", got.Status)
	}
	if !contains(w.events(t, c.ID), collections.EventStalePending) {
		t.Fatalf("stale event missing: %v", w.events(t, c.ID))
	}
	w.assertNoMoney(t, c)
}

func TestExpiryReportedByTheOperatorExpiresTheCollection(t *testing.T) {
	w := newWorld(t, 150)
	c := w.create(t, "09")
	w.clock.advance(collections.DefaultConfig().PendingCheckAfter + simDelay)
	if _, err := w.svc.Sweep(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	if got := w.reload(t, c.ID); got.Status != collections.StatusExpired || got.CompletedAt == nil {
		t.Fatalf("got %+v", got)
	}
	w.assertNoMoney(t, c)
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestEveryTransitionLeavesAnAuditEvent(t *testing.T) {
	w := newWorld(t, 0)
	c := w.create(t, "00")
	w.clock.advance(simDelay + 1)
	w.deliverCallbacks(t)
	got := w.events(t, c.ID)
	want := []string{collections.EventInitiated, collections.EventConfirmed}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("events = %v, want %v", got, want)
	}
}
