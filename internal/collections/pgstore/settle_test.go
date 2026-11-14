package pgstore_test

import (
	"context"
	"testing"

	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/ledger"
)

func (w *world) settle(t *testing.T) collections.SettleResult {
	t.Helper()
	res, err := w.svc.Settle(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// succeeded runs a collection to success and returns it.
func (w *world) succeeded(t *testing.T) collections.Collection {
	t.Helper()
	c := w.create(t, "00")
	w.clock.advance(simDelay + 1)
	w.deliverCallbacks(t)
	return w.reload(t, c.ID)
}

func TestFundsBecomeAvailableOnlyAfterTheSettlementDelay(t *testing.T) {
	w := newWorld(t, 150)
	c := w.succeeded(t)

	w.clock.advance(collections.DefaultConfig().SettlementDelay - 2*simDelay)
	if res := w.settle(t); res.Settled != 0 {
		t.Fatalf("too early: %+v", res)
	}
	w.clock.advance(3 * simDelay)
	if res := w.settle(t); res.Settled < 1 || res.Failed != 0 {
		t.Fatalf("after the delay: %+v", res)
	}
	if got := w.balance(t, ledger.MerchantAvailable(c.MerchantID)); got != 9850 {
		t.Fatalf("available = %d, want 9850", got)
	}
	if got := w.balance(t, ledger.MerchantPending(c.MerchantID)); got != 0 {
		t.Fatalf("pending = %d, want 0", got)
	}
}

func TestSettlingTwiceNeverReleasesTheFundsTwice(t *testing.T) {
	w := newWorld(t, 150)
	c := w.succeeded(t)
	w.clock.advance(collections.DefaultConfig().SettlementDelay + simDelay)
	w.settle(t)
	if res := w.settle(t); res.Settled != 0 {
		t.Fatalf("a settled collection must not settle again: %+v", res)
	}
	w.settle(t)
	if got := w.balance(t, ledger.MerchantAvailable(c.MerchantID)); got != 9850 {
		t.Fatalf("available = %d", got)
	}
}

func TestOnlySucceededCollectionsSettle(t *testing.T) {
	w := newWorld(t, 150)
	declined := w.create(t, "01")
	w.clock.advance(simDelay + 1)
	w.deliverCallbacks(t)
	w.clock.advance(collections.DefaultConfig().SettlementDelay)
	w.settle(t)
	_, err := w.ledger.Balance(context.Background(), ledger.MerchantAvailable(declined.MerchantID))
	if err == nil {
		t.Fatal("no available account should exist")
	}
}

func TestACollectionWhoseFeeEqualsItsAmountSettlesNothing(t *testing.T) {
	w := newWorld(t, 10_000) // 100 percent fee
	c := w.succeeded(t)
	w.clock.advance(collections.DefaultConfig().SettlementDelay + simDelay)
	if res := w.settle(t); res.Settled < 1 || res.Failed != 0 {
		t.Fatalf("got %+v", res)
	}
	if res := w.settle(t); res.Settled != 0 {
		t.Fatalf("it must be marked settled: %+v", res)
	}
	_ = c
}
