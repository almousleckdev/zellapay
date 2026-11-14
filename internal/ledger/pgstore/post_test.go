package pgstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/zellapay/zellapay/internal/ledger"
)

func TestPostUpdatesDerivedBalances(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.deposit(t, 1000)

	got, err := f.store.Balance(ctx, f.merchant)
	if err != nil || got != 1000 {
		t.Fatalf("merchant balance = %d, %v", got, err)
	}
	got, err = f.store.Balance(ctx, f.operator)
	if err != nil || got != 1000 {
		t.Fatalf("operator balance = %d, %v", got, err)
	}
}

func TestPostReplayWritesNothing(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.deposit(t, 500)
	key := uid("k")

	first, err := f.store.Post(ctx, f.transfer(key, 200))
	if err != nil || first.Replayed {
		t.Fatalf("first = %+v, %v", first, err)
	}
	second, err := f.store.Post(ctx, f.transfer(key, 200))
	if err != nil || !second.Replayed {
		t.Fatalf("second = %+v, %v", second, err)
	}
	if second.JournalID != first.JournalID {
		t.Fatal("replay must return the original journal")
	}
	if bal, _ := f.store.Balance(ctx, f.merchant); bal != 300 {
		t.Fatalf("balance = %d, want 300", bal)
	}
}

func TestPostRejectsReusedKeyWithDifferentBody(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.deposit(t, 500)
	key := uid("k")
	if _, err := f.store.Post(ctx, f.transfer(key, 200)); err != nil {
		t.Fatal(err)
	}
	_, err := f.store.Post(ctx, f.transfer(key, 201))
	if !errors.Is(err, ledger.ErrIdempotencyConflict) {
		t.Fatalf("got %v", err)
	}
}

func TestPostRejectsOverdraft(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.deposit(t, 100)

	_, err := f.store.Post(ctx, f.transfer(uid("k"), 101))
	if !errors.Is(err, ledger.ErrInsufficientFunds) {
		t.Fatalf("got %v", err)
	}
	if bal, _ := f.store.Balance(ctx, f.merchant); bal != 100 {
		t.Fatalf("balance changed to %d after rejected post", bal)
	}
}

func TestPostRejectsUnknownAccount(t *testing.T) {
	f := newFixture(t)
	j := f.transfer(uid("k"), 10)
	j.Entries[1].Account = "does-not-exist"
	if _, err := f.store.Post(context.Background(), j); !errors.Is(err, ledger.ErrUnknownAccount) {
		t.Fatalf("got %v", err)
	}
}

func TestPostRejectsUnbalancedJournal(t *testing.T) {
	f := newFixture(t)
	j := f.transfer(uid("k"), 10)
	j.Entries[1].Amount = 9
	if _, err := f.store.Post(context.Background(), j); !errors.Is(err, ledger.ErrUnbalanced) {
		t.Fatalf("got %v", err)
	}
}
