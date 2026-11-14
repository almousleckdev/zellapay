package pgstore

import (
	"context"
	"fmt"
	"sort"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/ledger/pgstore/sqlcgen"
)

// CreateAccount registers a new account.
func (s *Store) CreateAccount(ctx context.Context, a ledger.Account) error {
	if !a.Type.Valid() {
		return fmt.Errorf("%w: type %q", ledger.ErrInvalidJournal, a.Type)
	}
	return sqlcgen.New(s.pool).CreateAccount(ctx, sqlcgen.CreateAccountParams{
		ID:             a.ID,
		Code:           a.Code,
		Type:           string(a.Type),
		Currency:       string(a.Currency),
		AllowOverdraft: a.AllowOverdraft,
	})
}

// lockAccounts row-locks every account in sorted order, which prevents
// deadlocks between concurrent journals that touch overlapping accounts.
func lockAccounts(
	ctx context.Context, q *sqlcgen.Queries, j ledger.Journal,
) (map[string]sqlcgen.LockAccountsRow, error) {
	ids := accountIDs(j)
	rows, err := q.LockAccounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	if len(rows) != len(ids) {
		return nil, ledger.ErrUnknownAccount
	}
	byID := make(map[string]sqlcgen.LockAccountsRow, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}
	return byID, nil
}

func accountIDs(j ledger.Journal) []string {
	seen := make(map[string]struct{}, len(j.Entries))
	ids := make([]string, 0, len(j.Entries))
	for _, e := range j.Entries {
		if _, ok := seen[e.Account]; !ok {
			seen[e.Account] = struct{}{}
			ids = append(ids, e.Account)
		}
	}
	sort.Strings(ids)
	return ids
}

// EnsureAccount creates the account if it does not exist. It is safe to call
// concurrently and repeatedly. An existing account with a different type or
// currency is an error, never silently reused.
func (s *Store) EnsureAccount(ctx context.Context, a ledger.Account) error {
	q := sqlcgen.New(s.pool)
	if err := q.EnsureAccount(ctx, sqlcgen.EnsureAccountParams{
		ID: a.ID, Code: a.ID, Type: string(a.Type),
		Currency: string(a.Currency), AllowOverdraft: a.AllowOverdraft,
	}); err != nil {
		return err
	}
	got, err := q.GetAccount(ctx, a.ID)
	if err != nil {
		return err
	}
	if got.Type != string(a.Type) || got.Currency != string(a.Currency) ||
		got.AllowOverdraft != a.AllowOverdraft {
		return fmt.Errorf("%w: %s", ledger.ErrAccountMismatch, a.ID)
	}
	return nil
}
