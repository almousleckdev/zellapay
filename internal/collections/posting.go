package collections

import (
	"context"
	"fmt"

	"github.com/zellapay/zellapay/internal/ledger"
)

// journal builds the ledger entries for a succeeded collection: the
// operator owes us the full amount, the merchant is credited the amount
// less our fee, and the fee is revenue.
func (c Collection) journal() ledger.Journal {
	entries := []ledger.Entry{
		c.entry(ledger.OperatorSettlement(c.Operator), ledger.Debit, c.Amount),
	}
	if net := c.Amount - c.Fee; net > 0 {
		entries = append(entries, c.entry(ledger.MerchantPending(c.MerchantID), ledger.Credit, net))
	}
	if c.Fee > 0 {
		entries = append(entries, c.entry(ledger.Fees(c.Currency), ledger.Credit, c.Fee))
	}
	return ledger.Journal{
		ID:             "jnl_" + c.ID,
		IdempotencyKey: "collection:" + c.ID,
		Kind:           "collection",
		Reference:      c.ID,
		Entries:        entries,
	}
}

func (c Collection) entry(account string, side ledger.Side, amount int64) ledger.Entry {
	return ledger.Entry{
		Account: account, Side: side, Amount: amount, Currency: ledger.Currency(c.Currency),
	}
}

// postToLedger records the money in the books. It is idempotent: the key is
// derived from the collection ID, so a retry or a duplicate callback changes
// nothing. It runs before the status changes, so a crash can leave money
// recorded with the status still open, which the next check repairs, but
// never a succeeded status without the money.
func (s *Service) postToLedger(ctx context.Context, c Collection) error {
	if err := s.ensureAccounts(ctx, c); err != nil {
		return fmt.Errorf("ensure accounts: %w", err)
	}
	if _, err := s.d.Ledger.Post(ctx, c.journal()); err != nil {
		return fmt.Errorf("post journal: %w", err)
	}
	return nil
}

func (s *Service) ensureAccounts(ctx context.Context, c Collection) error {
	cur := ledger.Currency(c.Currency)
	accounts := []ledger.Account{
		{
			ID: ledger.OperatorSettlement(c.Operator), Type: ledger.Asset,
			Currency: cur, AllowOverdraft: true,
		},
		{ID: ledger.MerchantPending(c.MerchantID), Type: ledger.Liability, Currency: cur},
		{ID: ledger.Fees(c.Currency), Type: ledger.Revenue, Currency: cur},
	}
	for _, a := range accounts {
		if err := s.d.Accounts.EnsureAccount(ctx, a); err != nil {
			return err
		}
	}
	return nil
}
