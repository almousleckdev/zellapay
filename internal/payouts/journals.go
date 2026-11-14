package payouts

import (
	"context"
	"fmt"

	"github.com/zellapay/zellapay/internal/ledger"
)

// resolutionKey is shared by the capture and the release journal of a
// payout. Because the ledger refuses a second journal under the same key
// with different content, a payout can be settled in exactly one way: it
// can never be both paid out and refunded, whatever an operator reports.
func resolutionKey(id string) string { return "payout:resolution:" + id }

func (p Payout) entry(account string, side ledger.Side, amount int64) ledger.Entry {
	return ledger.Entry{
		Account: account, Side: side, Amount: amount, Currency: ledger.Currency(p.Currency),
	}
}

// holdJournal reserves the amount and fee: available to hold.
func (p Payout) holdJournal() ledger.Journal {
	return ledger.Journal{
		ID: "jnl_hold_" + p.ID, IdempotencyKey: "payout:hold:" + p.ID,
		Kind: "payout_hold", Reference: p.ID,
		Entries: ledger.Transfer(ledger.MerchantAvailable(p.MerchantID),
			ledger.MerchantHold(p.MerchantID), p.Total(), ledger.Currency(p.Currency)),
	}
}

// captureJournal records money leaving: the hold is spent, the operator
// balance falls by the amount, and the fee becomes revenue.
func (p Payout) captureJournal() ledger.Journal {
	entries := []ledger.Entry{
		p.entry(ledger.MerchantHold(p.MerchantID), ledger.Debit, p.Total()),
		p.entry(ledger.OperatorSettlement(p.Operator), ledger.Credit, p.Amount),
	}
	if p.Fee > 0 {
		entries = append(entries, p.entry(ledger.Fees(p.Currency), ledger.Credit, p.Fee))
	}
	return ledger.Journal{
		ID: "jnl_capture_" + p.ID, IdempotencyKey: resolutionKey(p.ID),
		Kind: "payout_capture", Reference: p.ID, Entries: entries,
	}
}

// releaseJournal returns the held funds: hold back to available.
func (p Payout) releaseJournal() ledger.Journal {
	return ledger.Journal{
		ID: "jnl_release_" + p.ID, IdempotencyKey: resolutionKey(p.ID),
		Kind: "payout_release", Reference: p.ID,
		Entries: ledger.Transfer(ledger.MerchantHold(p.MerchantID),
			ledger.MerchantAvailable(p.MerchantID), p.Total(), ledger.Currency(p.Currency)),
	}
}

// ensureAccounts creates the accounts every payout journal touches.
func (s *Service) ensureAccounts(ctx context.Context, p Payout) error {
	cur := ledger.Currency(p.Currency)
	accounts := []ledger.Account{
		{ID: ledger.MerchantAvailable(p.MerchantID), Type: ledger.Liability, Currency: cur},
		{ID: ledger.MerchantHold(p.MerchantID), Type: ledger.Liability, Currency: cur},
		{ID: ledger.Fees(p.Currency), Type: ledger.Revenue, Currency: cur},
		{
			ID: ledger.OperatorSettlement(p.Operator), Type: ledger.Asset,
			Currency: cur, AllowOverdraft: true,
		},
	}
	for _, a := range accounts {
		if err := s.d.Accounts.EnsureAccount(ctx, a); err != nil {
			return fmt.Errorf("ensure account %s: %w", a.ID, err)
		}
	}
	return nil
}

// post writes a journal, wrapping the error with what it was for.
func (s *Service) post(ctx context.Context, j ledger.Journal) error {
	if _, err := s.d.Ledger.Post(ctx, j); err != nil {
		return fmt.Errorf("post %s: %w", j.Kind, err)
	}
	return nil
}
