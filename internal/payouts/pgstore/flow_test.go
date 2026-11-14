package pgstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/payouts"
)

func TestASuccessfulPayoutHoldsThenSpendsTheFunds(t *testing.T) {
	w := newWorld(t, 100, 20_000) // 1 percent payout fee
	p := w.create(t, "00", 10_000)
	if p.Status != payouts.StatusPending || p.Fee != 100 || p.Operator != "simulator" {
		t.Fatalf("unexpected payout %+v", p)
	}
	if w.available(t) != 9_900 || w.held(t) != 10_100 {
		t.Fatalf("after the hold: available %d, held %d", w.available(t), w.held(t))
	}

	w.clock.advance(simDelay + 1)
	w.deliverCallbacks(t)
	if got := w.reload(t, p.ID); got.Status != payouts.StatusSucceeded || got.CompletedAt == nil {
		t.Fatalf("got %+v", got)
	}
	if w.available(t) != 9_900 || w.held(t) != 0 {
		t.Fatalf("after capture: available %d, held %d", w.available(t), w.held(t))
	}
}

func TestDuplicateCallbacksNeverCaptureTwice(t *testing.T) {
	w := newWorld(t, 0, 20_000)
	p := w.create(t, "05", 10_000)
	w.clock.advance(simDelay + 1)
	if n := w.deliverCallbacks(t); n < 2 {
		t.Fatalf("want a duplicate callback, got %d", n)
	}
	if got := w.reload(t, p.ID).Status; got != payouts.StatusSucceeded {
		t.Fatalf("status = %s", got)
	}
	if w.held(t) != 0 || w.available(t) != 10_000 {
		t.Fatalf("available %d, held %d", w.available(t), w.held(t))
	}
}

func TestADeclinedPayoutReturnsTheFunds(t *testing.T) {
	w := newWorld(t, 100, 20_000)
	p := w.create(t, "01", 10_000)
	w.clock.advance(simDelay + 1)
	w.deliverCallbacks(t)
	got := w.reload(t, p.ID)
	if got.Status != payouts.StatusFailed || got.FailureCode != "CUSTOMER_DECLINED" {
		t.Fatalf("got %+v", got)
	}
	if w.available(t) != 20_000 || w.held(t) != 0 {
		t.Fatalf("funds must be fully returned: available %d, held %d", w.available(t), w.held(t))
	}
}

func TestRefusalByTheOperatorReturnsTheFundsImmediately(t *testing.T) {
	w := newWorld(t, 100, 20_000)
	p := w.create(t, "08", 10_000)
	if p.Status != payouts.StatusFailed || p.FailureCode != "rejected" {
		t.Fatalf("got %+v", p)
	}
	if w.available(t) != 20_000 || w.held(t) != 0 {
		t.Fatalf("available %d, held %d", w.available(t), w.held(t))
	}
}

func TestATimeoutKeepsTheFundsHeldUntilTheOperatorAnswers(t *testing.T) {
	w := newWorld(t, 0, 20_000)
	p := w.create(t, "03", 10_000)
	if p.Status != payouts.StatusAmbiguous {
		t.Fatalf("a timeout must be ambiguous, got %s", p.Status)
	}
	if w.held(t) != 10_000 || w.available(t) != 10_000 {
		t.Fatalf("funds must stay held: available %d, held %d", w.available(t), w.held(t))
	}
	w.clock.advance(payouts.DefaultConfig().AmbiguousCheckAfter + simDelay)
	if _, err := w.svc.Sweep(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	if got := w.reload(t, p.ID).Status; got != payouts.StatusSucceeded {
		t.Fatalf("status = %s", got)
	}
	if w.held(t) != 0 || w.available(t) != 10_000 {
		t.Fatalf("available %d, held %d", w.available(t), w.held(t))
	}
}

func TestInsufficientFundsRejectsAndStaysRejectedOnRetry(t *testing.T) {
	w := newWorld(t, 0, 5_000)
	in := w.input("00", "key-poor-1", 10_000)
	for i := 0; i < 3; i++ {
		p, err := w.svc.Create(context.Background(), in)
		if !errors.Is(err, payouts.ErrInsufficientFunds) || p.Status != payouts.StatusRejected {
			t.Fatalf("attempt %d: %+v, %v", i, p, err)
		}
	}
	if w.available(t) != 5_000 || w.held(t) != 0 {
		t.Fatalf("a rejected payout must not move money: available %d, held %d",
			w.available(t), w.held(t))
	}
}

func TestAMerchantWithNoBalanceAtAllIsRejectedNotBroken(t *testing.T) {
	w := newWorld(t, 0, 0)
	_, err := w.svc.Create(context.Background(), w.input("00", "key-empty-1", 1_000))
	if !errors.Is(err, payouts.ErrInsufficientFunds) {
		t.Fatalf("got %v", err)
	}
}

func TestTheFeeCountsTowardsTheFundsNeeded(t *testing.T) {
	w := newWorld(t, 1_000, 10_000) // 10 percent fee
	_, err := w.svc.Create(context.Background(), w.input("00", "key-fee-1", 10_000))
	if !errors.Is(err, payouts.ErrInsufficientFunds) {
		t.Fatalf("amount plus fee exceeds the balance, got %v", err)
	}
	if _, err := w.svc.Create(context.Background(), w.input("00", "key-fee-2", 9_000)); err != nil {
		t.Fatalf("9000 plus a 900 fee fits in 10000: %v", err)
	}
	_ = ledger.Fees
}
