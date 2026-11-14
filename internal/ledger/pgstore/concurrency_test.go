package pgstore_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/zellapay/zellapay/internal/ledger"
)

// Many concurrent transfers against a limited balance must succeed exactly
// as often as the funds allow and never overdraw the account.
func TestConcurrentTransfersNeverOverdraw(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	const funds, amount, attempts = 1000, 50, 60
	f.deposit(t, funds)

	var ok, rejected atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.store.Post(ctx, f.transfer(uid("k"), amount))
			switch {
			case err == nil:
				ok.Add(1)
			case errors.Is(err, ledger.ErrInsufficientFunds):
				rejected.Add(1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if ok.Load() != funds/amount {
		t.Fatalf("succeeded %d, want %d", ok.Load(), funds/amount)
	}
	assertBalance(t, f, f.merchant, 0)
	assertBalance(t, f, f.other, funds)
}

// Concurrent requests that share an idempotency key post exactly once.
func TestConcurrentSameKeyPostsOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.deposit(t, 1000)
	key := uid("k")

	var fresh atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := f.store.Post(ctx, f.transfer(key, 100))
			if err != nil {
				t.Errorf("post: %v", err)
				return
			}
			if !res.Replayed {
				fresh.Add(1)
			}
		}()
	}
	wg.Wait()

	if fresh.Load() != 1 {
		t.Fatalf("%d fresh postings, want 1", fresh.Load())
	}
	assertBalance(t, f, f.merchant, 900)
}

func assertBalance(t *testing.T, f fixture, account string, want int64) {
	t.Helper()
	got, err := f.store.Balance(context.Background(), account)
	if err != nil || got != want {
		t.Fatalf("balance of %s = %d, %v; want %d", account, got, err, want)
	}
}
