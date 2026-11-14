package pgstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/ledger"
)

func (w *world) balance(t *testing.T, account string) int64 {
	t.Helper()
	got, err := w.ledger.Balance(context.Background(), account)
	if err != nil {
		t.Fatalf("balance of %s: %v", account, err)
	}
	return got
}

func (w *world) reload(t *testing.T, id string) collections.Collection {
	t.Helper()
	c, err := w.svc.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSuccessfulCollectionRecordsTheMoneyExactlyOnce(t *testing.T) {
	w := newWorld(t, 150)
	c := w.create(t, "00")
	if c.Status != collections.StatusPending || c.Operator != "simulator" || c.Fee != 150 {
		t.Fatalf("unexpected collection %+v", c)
	}
	w.clock.advance(simDelay + 1)
	if n := w.deliverCallbacks(t); n != 1 {
		t.Fatalf("want 1 callback, got %d", n)
	}
	if got := w.reload(t, c.ID).Status; got != collections.StatusSucceeded {
		t.Fatalf("status = %s", got)
	}
	w.assertBooks(t, c, wantBooks{pending: 9850, fees: 150, settlement: 10000})
}

// wantBooks are the expected ledger balances. Fees and settlement accounts
// are shared by every test, so they are checked as lower bounds.
type wantBooks struct{ pending, fees, settlement int64 }

func (w *world) assertBooks(t *testing.T, c collections.Collection, want wantBooks) {
	t.Helper()
	if got := w.balance(t, ledger.MerchantPending(c.MerchantID)); got != want.pending {
		t.Errorf("merchant pending = %d, want %d", got, want.pending)
	}
	if got := w.balance(t, ledger.Fees("XAF")); got < want.fees {
		t.Errorf("fees = %d, want at least %d", got, want.fees)
	}
	if got := w.balance(t, ledger.OperatorSettlement("simulator")); got < want.settlement {
		t.Errorf("operator settlement = %d, want at least %d", got, want.settlement)
	}
}

func TestDuplicateCallbacksNeverPostTwice(t *testing.T) {
	w := newWorld(t, 150)
	c := w.create(t, "05")
	w.clock.advance(simDelay + 1)
	if n := w.deliverCallbacks(t); n != 2 {
		t.Fatalf("want 2 callbacks, got %d", n)
	}
	if got := w.balance(t, ledger.MerchantPending(c.MerchantID)); got != 9850 {
		t.Fatalf("merchant pending = %d, want 9850 after a duplicate callback", got)
	}
	if res, err := w.svc.Sweep(context.Background(), 100); err != nil || res.Changed != 0 {
		t.Fatalf("sweep after completion: %+v, %v", res, err)
	}
}

func TestDeclinedCollectionFailsWithoutMovingMoney(t *testing.T) {
	w := newWorld(t, 150)
	c := w.create(t, "01")
	w.clock.advance(simDelay + 1)
	w.deliverCallbacks(t)
	got := w.reload(t, c.ID)
	if got.Status != collections.StatusFailed || got.FailureCode != "CUSTOMER_DECLINED" {
		t.Fatalf("got %+v", got)
	}
	_, err := w.ledger.Balance(context.Background(), ledger.MerchantPending(c.MerchantID))
	if !errors.Is(err, ledger.ErrUnknownAccount) {
		t.Fatalf("no ledger account should exist, got %v", err)
	}
}

func TestOperatorRejectionFailsImmediately(t *testing.T) {
	w := newWorld(t, 0)
	c := w.create(t, "08")
	if c.Status != collections.StatusFailed || c.FailureCode != "rejected" {
		t.Fatalf("got %+v", c)
	}
}

func TestTimeoutIsResolvedByAskingTheOperator(t *testing.T) {
	w := newWorld(t, 150)
	c := w.create(t, "03")
	if c.Status != collections.StatusAmbiguous {
		t.Fatalf("a timeout must be ambiguous, got %s", c.Status)
	}
	w.clock.advance(collections.DefaultConfig().AmbiguousCheckAfter + simDelay)
	if res, err := w.svc.Sweep(context.Background(), 100); err != nil || res.Changed != 1 {
		t.Fatalf("sweep: %+v, %v", res, err)
	}
	if got := w.reload(t, c.ID).Status; got != collections.StatusSucceeded {
		t.Fatalf("status = %s", got)
	}
	w.assertBooks(t, c, wantBooks{pending: 9850, fees: 150, settlement: 10000})
}
