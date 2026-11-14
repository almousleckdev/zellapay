package pgstore_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/ledger/pgstore"
)

var seq atomic.Int64

// uid returns a unique identifier so tests never collide on shared tables.
func uid(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, seq.Add(1))
}

type fixture struct {
	store    *pgstore.Store
	operator string // asset, may be overdrawn
	merchant string // liability, may not be overdrawn
	other    string // liability, may not be overdrawn
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	store := pgstore.New(newPool(t))
	f := fixture{
		store:    store,
		operator: uid("op"),
		merchant: uid("m"),
		other:    uid("m"),
	}
	ctx := context.Background()
	accounts := []ledger.Account{
		{
			ID: f.operator, Code: f.operator, Type: ledger.Asset,
			Currency: "XAF", AllowOverdraft: true,
		},
		{ID: f.merchant, Code: f.merchant, Type: ledger.Liability, Currency: "XAF"},
		{ID: f.other, Code: f.other, Type: ledger.Liability, Currency: "XAF"},
	}
	for _, a := range accounts {
		if err := store.CreateAccount(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

// deposit credits the merchant from the operator account.
func (f fixture) deposit(t *testing.T, amount int64) {
	t.Helper()
	j := ledger.Journal{
		ID: uid("j"), IdempotencyKey: uid("k"), Kind: "collection", Reference: uid("r"),
		Entries: []ledger.Entry{
			{Account: f.operator, Side: ledger.Debit, Amount: amount, Currency: "XAF"},
			{Account: f.merchant, Side: ledger.Credit, Amount: amount, Currency: "XAF"},
		},
	}
	if _, err := f.store.Post(context.Background(), j); err != nil {
		t.Fatal(err)
	}
}

// transfer moves amount from the merchant to the other account.
func (f fixture) transfer(key string, amount int64) ledger.Journal {
	return ledger.Journal{
		ID: uid("j"), IdempotencyKey: key, Kind: "transfer", Reference: key,
		Entries: []ledger.Entry{
			{Account: f.merchant, Side: ledger.Debit, Amount: amount, Currency: "XAF"},
			{Account: f.other, Side: ledger.Credit, Amount: amount, Currency: "XAF"},
		},
	}
}
