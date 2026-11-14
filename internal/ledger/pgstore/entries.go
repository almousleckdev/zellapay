package pgstore

import (
	"context"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/ledger/pgstore/sqlcgen"
)

func insertEntries(ctx context.Context, q *sqlcgen.Queries, j ledger.Journal) error {
	n := len(j.Entries)
	p := sqlcgen.InsertEntriesParams{
		JournalID:  j.ID,
		Accounts:   make([]string, 0, n),
		Sides:      make([]string, 0, n),
		Amounts:    make([]int64, 0, n),
		Currencies: make([]string, 0, n),
	}
	for _, e := range j.Entries {
		p.Accounts = append(p.Accounts, e.Account)
		p.Sides = append(p.Sides, e.Side.String())
		p.Amounts = append(p.Amounts, e.Amount)
		p.Currencies = append(p.Currencies, string(e.Currency))
	}
	return q.InsertEntries(ctx, p)
}
