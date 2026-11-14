package pgstore

import (
	"context"
	"fmt"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/ledger/pgstore/sqlcgen"
)

// checkOverdraft verifies, while the accounts are locked, that no account
// without overdraft permission ends with a negative natural balance.
func checkOverdraft(
	ctx context.Context, q *sqlcgen.Queries, accounts map[string]sqlcgen.LockAccountsRow,
) error {
	ids := make([]string, 0, len(accounts))
	for id, a := range accounts {
		if !a.AllowOverdraft {
			ids = append(ids, id)
		}
	}
	totals, err := q.AccountTotals(ctx, ids)
	if err != nil {
		return err
	}
	for _, t := range totals {
		typ := ledger.AccountType(accounts[t.AccountID].Type)
		if ledger.NormalBalance(typ, t.Debits, t.Credits) < 0 {
			return fmt.Errorf("%w: account %s", ledger.ErrInsufficientFunds, t.AccountID)
		}
	}
	return nil
}
