// Package ledgertest helps tests put money into the ledger. It must only be
// imported from tests.
package ledgertest

import (
	"context"
	"testing"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/platform/ids"
)

// Store is a ledger that can create accounts and post journals.
type Store interface {
	ledger.Poster
	EnsureAccount(ctx context.Context, a ledger.Account) error
}

// FundMerchant credits a merchant's available balance in XAF, as settlement
// would, paid for by the simulator operator's balance.
func FundMerchant(t *testing.T, s Store, merchantID string, amount int64) {
	t.Helper()
	ctx := context.Background()
	available := ledger.MerchantAvailable(merchantID)
	operator := ledger.OperatorSettlement("simulator")
	for _, a := range []ledger.Account{
		{ID: available, Type: ledger.Liability, Currency: "XAF"},
		{ID: operator, Type: ledger.Asset, Currency: "XAF", AllowOverdraft: true},
	} {
		if err := s.EnsureAccount(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	_, err := s.Post(ctx, ledger.Journal{
		ID: ids.New("jnl_"), IdempotencyKey: ids.New("fund_"),
		Kind: "test_funding", Reference: "fund",
		Entries: ledger.Transfer(operator, available, amount, "XAF"),
	})
	if err != nil {
		t.Fatal(err)
	}
}
