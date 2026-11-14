package pgstore_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/operators"
	"github.com/zellapay/zellapay/internal/platform/ids"
)

func TestCreateIsIdempotentPerKey(t *testing.T) {
	w := newWorld(t, 150)
	in := w.input("00", "key-fixed-1")
	first, err := w.svc.Create(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := w.svc.Create(context.Background(), in)
	if err != nil || second.ID != first.ID {
		t.Fatalf("repeat must return the original: %+v, %v", second, err)
	}
	in.Amount = 20000
	_, err = w.svc.Create(context.Background(), in)
	if !errors.Is(err, collections.ErrIdempotencyConflict) {
		t.Fatalf("different body, same key: got %v", err)
	}
}

func TestConcurrentCreatesWithOneKeyMakeOneCollection(t *testing.T) {
	w := newWorld(t, 150)
	in := w.input("00", "key-race-1")
	ids := make(chan string, 12)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := w.svc.Create(context.Background(), in)
			if err != nil {
				t.Errorf("create: %v", err)
				return
			}
			ids <- c.ID
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		seen[id] = true
	}
	if len(seen) != 1 {
		t.Fatalf("%d distinct collections, want 1", len(seen))
	}
}

func TestUnsupportedNumberIsRefusedBeforeAnythingIsStored(t *testing.T) {
	w := newWorld(t, 0)
	in := w.input("00", "key-nr-1")
	in.Phone = "+33612345678"
	_, err := w.svc.Create(context.Background(), in)
	if !errors.Is(err, collections.ErrUnsupportedNumber) {
		t.Fatalf("got %v", err)
	}
}

func TestInvalidInputIsRefused(t *testing.T) {
	w := newWorld(t, 0)
	mutations := map[string]func(*collections.CreateInput){
		"zero amount":  func(i *collections.CreateInput) { i.Amount = 0 },
		"bad phone":    func(i *collections.CreateInput) { i.Phone = "0707" },
		"bad currency": func(i *collections.CreateInput) { i.Currency = "xaf" },
		"no key":       func(i *collections.CreateInput) { i.IdempotencyKey = "" },
	}
	for name, mutate := range mutations {
		in := w.input("00", "key-inv")
		mutate(&in)
		_, err := w.svc.Create(context.Background(), in)
		if !errors.Is(err, collections.ErrInvalidInput) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

// A crash after the row is stored but before the operator is called must
// not leave money in limbo.
func TestStoredButNeverSentRequestIsFailedByTheSweeper(t *testing.T) {
	w := newWorld(t, 0)
	id := w.storeWithoutInitiating(t, "00")
	w.clock.advance(collections.DefaultConfig().CreatedGrace + time.Second)
	if _, err := w.svc.Sweep(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	got := w.reload(t, id)
	if got.Status != collections.StatusFailed || got.FailureCode != "operator_never_received" {
		t.Fatalf("got %+v", got)
	}
}

// A crash after the operator accepted but before we recorded it: the
// operator knows the reference, so asking by reference finds the truth.
func TestSentButNeverRecordedRequestIsResolvedByReference(t *testing.T) {
	w := newWorld(t, 0)
	id := w.storeWithoutInitiating(t, "00")
	_, err := w.sim.InitiateCollection(context.Background(), operators.CollectionRequest{
		Reference: id, Phone: phone("00"), Amount: 10000, Currency: "XAF",
	})
	if err != nil {
		t.Fatal(err)
	}
	w.clock.advance(collections.DefaultConfig().CreatedGrace + simDelay)
	if _, err := w.svc.Sweep(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	if got := w.reload(t, id).Status; got != collections.StatusSucceeded {
		t.Fatalf("status = %s", got)
	}
}

func (w *world) storeWithoutInitiating(t *testing.T, suffix string) string {
	t.Helper()
	now := w.clock.Now()
	grace := now.Add(collections.DefaultConfig().CreatedGrace)
	c, _, err := w.store.Create(context.Background(), collections.NewCollection{
		Phone: phone(suffix),
		Collection: collections.Collection{
			ID: ids.New("col_"), MerchantID: w.merchant.ID,
			IdempotencyKey: ids.New("key_"), MerchantReference: "R",
			Status: collections.StatusCreated, Amount: 10000, Currency: "XAF",
			PhoneMasked: "+241 07 ** ** " + suffix, Operator: "simulator",
			CreatedAt: now, UpdatedAt: now, NextCheckAt: &grace,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c.ID
}
