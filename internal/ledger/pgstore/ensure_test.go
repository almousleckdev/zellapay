package pgstore_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/ledger/pgstore"
)

func TestEnsureAccountIsIdempotent(t *testing.T) {
	store := pgstore.New(newPool(t))
	acct := ledger.Account{ID: uid("ens"), Type: ledger.Liability, Currency: "XAF"}
	for i := 0; i < 3; i++ {
		if err := store.EnsureAccount(context.Background(), acct); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
}

func TestEnsureAccountRefusesADifferentDefinition(t *testing.T) {
	store := pgstore.New(newPool(t))
	acct := ledger.Account{ID: uid("ens"), Type: ledger.Liability, Currency: "XAF"}
	if err := store.EnsureAccount(context.Background(), acct); err != nil {
		t.Fatal(err)
	}
	for name, other := range map[string]ledger.Account{
		"type":      {ID: acct.ID, Type: ledger.Asset, Currency: "XAF"},
		"currency":  {ID: acct.ID, Type: ledger.Liability, Currency: "EUR"},
		"overdraft": {ID: acct.ID, Type: ledger.Liability, Currency: "XAF", AllowOverdraft: true},
	} {
		err := store.EnsureAccount(context.Background(), other)
		if !errors.Is(err, ledger.ErrAccountMismatch) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestEnsureAccountIsSafeConcurrently(t *testing.T) {
	store := pgstore.New(newPool(t))
	acct := ledger.Account{ID: uid("ens"), Type: ledger.Asset, Currency: "XAF"}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := store.EnsureAccount(context.Background(), acct); err != nil {
				t.Errorf("ensure: %v", err)
			}
		}()
	}
	wg.Wait()
}
