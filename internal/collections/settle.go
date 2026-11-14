package collections

import (
	"context"
	"fmt"

	"github.com/zellapay/zellapay/internal/ledger"
)

// SettleResult counts what one settlement pass did.
type SettleResult struct {
	Settled int
	Failed  int
}

// Settle releases collected funds to merchants. Once a collection has been
// succeeded for the settlement delay, its net amount moves from the
// merchant's pending balance to their available balance.
//
// The journal is keyed on the collection, so a crash between posting it and
// marking the collection settled only repeats a harmless replay.
func (s *Service) Settle(ctx context.Context, limit int) (SettleResult, error) {
	cutoff := s.now().Add(-s.cfg.SettlementDelay)
	due, err := s.d.Repo.Unsettled(ctx, cutoff, limit)
	if err != nil {
		return SettleResult{}, err
	}
	var res SettleResult
	for _, c := range due {
		if err := s.settleOne(ctx, c); err != nil {
			res.Failed++
			s.d.Log.Error("settlement failed", "collection_id", c.ID, "err", err)
			continue
		}
		res.Settled++
	}
	return res, nil
}

func (s *Service) settleOne(ctx context.Context, c Collection) error {
	if net := c.Amount - c.Fee; net > 0 {
		if err := s.d.Accounts.EnsureAccount(ctx, ledger.Account{
			ID: ledger.MerchantAvailable(c.MerchantID), Type: ledger.Liability,
			Currency: ledger.Currency(c.Currency),
		}); err != nil {
			return fmt.Errorf("ensure account: %w", err)
		}
		if _, err := s.d.Ledger.Post(ctx, c.settlementJournal(net)); err != nil {
			return fmt.Errorf("post settlement: %w", err)
		}
	}
	return s.d.Repo.MarkSettled(ctx, c.ID, s.now())
}

// settlementJournal moves net from pending to available.
func (c Collection) settlementJournal(net int64) ledger.Journal {
	return ledger.Journal{
		ID:             "jnl_settle_" + c.ID,
		IdempotencyKey: "settle:collection:" + c.ID,
		Kind:           "settlement",
		Reference:      c.ID,
		Entries: ledger.Transfer(ledger.MerchantPending(c.MerchantID),
			ledger.MerchantAvailable(c.MerchantID), net, ledger.Currency(c.Currency)),
	}
}
