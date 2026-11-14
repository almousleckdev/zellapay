package pgstore

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/ledger/pgstore/sqlcgen"
)

// Balance returns the account's natural balance derived from its entries.
func (s *Store) Balance(ctx context.Context, accountID string) (int64, error) {
	t, err := sqlcgen.New(s.pool).AccountBalanceTotals(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ledger.ErrUnknownAccount
	}
	if err != nil {
		return 0, err
	}
	return ledger.NormalBalance(ledger.AccountType(t.Type), t.Debits, t.Credits), nil
}
