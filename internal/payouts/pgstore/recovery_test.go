package pgstore_test

import (
	"context"
	"testing"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/payouts"
	"github.com/zellapay/zellapay/internal/platform/ids"
)

// stored inserts a payout directly, as the first step of Create would.
func (w *world) stored(t *testing.T, amount int64) payouts.Payout {
	t.Helper()
	now := w.clock.Now()
	grace := now.Add(payouts.DefaultConfig().CreatedGrace)
	p, _, err := w.store.Create(context.Background(), payouts.NewPayout{
		Phone: phone("00"),
		Payout: payouts.Payout{
			ID: ids.New("pay_"), MerchantID: w.merchant.ID, IdempotencyKey: ids.New("key_"),
			MerchantReference: "R", Status: payouts.StatusCreated, Amount: amount, Currency: "XAF",
			PhoneMasked: "+241 07 ** ** 00", Operator: "simulator",
			CreatedAt: now, UpdatedAt: now, NextCheckAt: &grace,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func (w *world) sweepAfterGrace(t *testing.T) {
	t.Helper()
	w.clock.advance(payouts.DefaultConfig().CreatedGrace + 1)
	if _, err := w.svc.Sweep(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
}

// A crash before the funds were held: nothing happened, so nothing is owed.
func TestStoredButNeverHeldPayoutEndsWithNoMoneyMoved(t *testing.T) {
	w := newWorld(t, 0, 20_000)
	p := w.stored(t, 10_000)
	w.sweepAfterGrace(t)
	got := w.reload(t, p.ID)
	if got.Status != payouts.StatusFailed || got.FailureCode != "abandoned" {
		t.Fatalf("got %+v", got)
	}
	if w.available(t) != 20_000 || w.held(t) != 0 {
		t.Fatalf("available %d, held %d", w.available(t), w.held(t))
	}
}

// A crash after the funds were held but before the status was saved: the
// hold exists, so the abandon must give it back.
func TestHeldFundsOfAnAbandonedPayoutAreReturned(t *testing.T) {
	w := newWorld(t, 0, 20_000)
	p := w.stored(t, 10_000)
	w.postHold(t, p)
	if w.held(t) != 10_000 {
		t.Fatal("setup: the hold must exist")
	}
	w.sweepAfterGrace(t)
	if got := w.reload(t, p.ID).Status; got != payouts.StatusFailed {
		t.Fatalf("status = %s", got)
	}
	if w.available(t) != 20_000 || w.held(t) != 0 {
		t.Fatalf("the hold must be returned: available %d, held %d", w.available(t), w.held(t))
	}
}

// A crash after the hold was saved but before the operator was asked: the
// operator has no record of it, so no money left and the hold is returned.
func TestHeldButNeverSentPayoutIsReleased(t *testing.T) {
	w := newWorld(t, 0, 20_000)
	p := w.stored(t, 10_000)
	ctx := context.Background()
	w.postHold(t, p)
	grace := w.clock.Now()
	markHeld := func(cur payouts.Payout) (payouts.Payout, payouts.Event, error) {
		cur.Status, cur.NextCheckAt = payouts.StatusHeld, &grace
		return cur, payouts.Event{}, nil
	}
	if _, err := w.store.Update(ctx, p.ID, markHeld); err != nil {
		t.Fatal(err)
	}
	w.clock.advance(1)
	if _, err := w.svc.Sweep(ctx, 100); err != nil {
		t.Fatal(err)
	}
	got := w.reload(t, p.ID)
	if got.Status != payouts.StatusFailed || got.FailureCode != "operator_never_received" {
		t.Fatalf("got %+v", got)
	}
	if w.available(t) != 20_000 || w.held(t) != 0 {
		t.Fatalf("available %d, held %d", w.available(t), w.held(t))
	}
}

// postHold reserves a payout's funds the way Create does.
func (w *world) postHold(t *testing.T, p payouts.Payout) {
	t.Helper()
	ctx := context.Background()
	if err := w.ledger.EnsureAccount(ctx, ledger.Account{
		ID: ledger.MerchantHold(w.merchant.ID), Type: ledger.Liability, Currency: "XAF",
	}); err != nil {
		t.Fatal(err)
	}
	_, err := w.ledger.Post(ctx, ledger.Journal{
		ID: "jnl_hold_" + p.ID, IdempotencyKey: "payout:hold:" + p.ID,
		Kind: "payout_hold", Reference: p.ID,
		Entries: ledger.Transfer(ledger.MerchantAvailable(w.merchant.ID),
			ledger.MerchantHold(w.merchant.ID), p.Amount, "XAF"),
	})
	if err != nil {
		t.Fatal(err)
	}
}
