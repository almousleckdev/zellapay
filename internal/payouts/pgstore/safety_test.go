package pgstore_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/payouts"
	"github.com/zellapay/zellapay/internal/platform/ids"
)

// Many payouts racing for a limited balance must succeed exactly as often
// as the funds allow, and the balance must never go negative.
func TestConcurrentPayoutsNeverOverspendTheBalance(t *testing.T) {
	w := newWorld(t, 0, 10_000)
	var accepted, refused atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := w.svc.Create(context.Background(), w.input("00", ids.New("key_"), 3_000))
			switch {
			case err == nil:
				accepted.Add(1)
			case errors.Is(err, payouts.ErrInsufficientFunds):
				refused.Add(1)
			default:
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 3 || refused.Load() != 9 {
		t.Fatalf("accepted %d, refused %d; 10000 funds three payouts of 3000",
			accepted.Load(), refused.Load())
	}
	if w.available(t) != 1_000 || w.held(t) != 9_000 {
		t.Fatalf("available %d, held %d", w.available(t), w.held(t))
	}
}

func TestOneKeyFromManyConcurrentRequestsPaysOnce(t *testing.T) {
	w := newWorld(t, 0, 20_000)
	in := w.input("00", "key-race-1", 5_000)
	ids := make(chan string, 12)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := w.svc.Create(context.Background(), in)
			if err != nil {
				t.Errorf("create: %v", err)
				return
			}
			ids <- p.ID
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		seen[id] = true
	}
	if len(seen) != 1 {
		t.Fatalf("%d distinct payouts, want 1", len(seen))
	}
	if w.held(t) != 5_000 || w.available(t) != 15_000 {
		t.Fatalf("funds must be held once: available %d, held %d", w.available(t), w.held(t))
	}
}

func TestReusingAKeyWithADifferentRequestIsRefused(t *testing.T) {
	w := newWorld(t, 0, 20_000)
	in := w.input("00", "key-reuse-1", 5_000)
	if _, err := w.svc.Create(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	in.Amount = 6_000
	_, err := w.svc.Create(context.Background(), in)
	if !errors.Is(err, payouts.ErrIdempotencyConflict) {
		t.Fatalf("got %v", err)
	}
}

func TestAmountAboveTheLimitIsRefusedBeforeAnythingIsStored(t *testing.T) {
	w := newWorld(t, 0, 50_000_000)
	max := payouts.DefaultConfig().MaxAmount
	_, err := w.svc.Create(context.Background(), w.input("00", "key-big-1", max+1))
	if !errors.Is(err, payouts.ErrAmountTooLarge) {
		t.Fatalf("got %v", err)
	}
	if w.available(t) != 50_000_000 {
		t.Fatalf("no money may move, available = %d", w.available(t))
	}
}

func TestAWrongAmountCallbackHoldsTheFundsForAPerson(t *testing.T) {
	w := newWorld(t, 0, 20_000)
	p := w.create(t, "07", 10_000)
	w.clock.advance(simDelay + 1)
	w.deliverCallbacks(t)
	got := w.reload(t, p.ID)
	held := got.Status == payouts.StatusAmbiguous &&
		got.FailureCode == payouts.CodeAmountMismatch && got.NextCheckAt == nil
	if !held {
		t.Fatalf("got %+v", got)
	}
	w.clock.advance(48 * time.Hour)
	if _, err := w.svc.Sweep(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	after := w.reload(t, p.ID)
	if after.Status != payouts.StatusAmbiguous || after.FailureCode != payouts.CodeAmountMismatch {
		t.Fatalf("a held payout must be left alone, got %+v", after)
	}
	if w.held(t) != 10_000 {
		t.Fatalf("the funds must stay held, got %d", w.held(t))
	}
}

func TestAPayoutStuckPendingIsFlaggedAndKeepsItsFundsHeld(t *testing.T) {
	w := newWorld(t, 0, 20_000)
	p := w.create(t, "04", 10_000)
	w.clock.advance(payouts.DefaultConfig().StaleAfter + time.Minute)
	if _, err := w.svc.Sweep(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	if got := w.reload(t, p.ID).Status; got != payouts.StatusPending {
		t.Fatalf("only the operator may end it, got %s", got)
	}
	if w.held(t) != 10_000 {
		t.Fatalf("held = %d", w.held(t))
	}
}

// A payout can be settled one way only: the ledger refuses to resolve the
// same payout a second, different way, whatever an operator later claims.
func TestAPaidPayoutCanNeverAlsoBeRefunded(t *testing.T) {
	w := newWorld(t, 0, 20_000)
	p := w.create(t, "00", 10_000)
	w.clock.advance(simDelay + 1)
	w.deliverCallbacks(t)
	if w.reload(t, p.ID).Status != payouts.StatusSucceeded {
		t.Fatal("setup: the payout must have succeeded")
	}
	refund := ledger.Journal{
		ID: "jnl_forced_release", IdempotencyKey: "payout:resolution:" + p.ID,
		Kind: "payout_release", Reference: p.ID,
		Entries: ledger.Transfer(ledger.MerchantHold(p.MerchantID),
			ledger.MerchantAvailable(p.MerchantID), 10_000, "XAF"),
	}
	_, err := w.ledger.Post(context.Background(), refund)
	if !errors.Is(err, ledger.ErrIdempotencyConflict) {
		t.Fatalf("the ledger must refuse a second resolution, got %v", err)
	}
	if w.available(t) != 10_000 {
		t.Fatalf("available = %d; the merchant must not be paid and refunded", w.available(t))
	}
}
