package payouts

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/operators/router"
	"github.com/zellapay/zellapay/internal/platform/ids"
	"github.com/zellapay/zellapay/internal/platform/money"
	"github.com/zellapay/zellapay/internal/platform/phone"
)

// CreateInput is a request to send money.
type CreateInput struct {
	MerchantID     string
	IdempotencyKey string
	Reference      string
	Phone          string // E.164
	Currency       string
	Description    string
	Amount         int64
}

func (in CreateInput) validate(max int64) error {
	switch {
	case in.MerchantID == "", in.IdempotencyKey == "", in.Reference == "":
		return fmt.Errorf("%w: missing field", ErrInvalidInput)
	case len(in.Reference) > 128 || len(in.Description) > 200:
		return fmt.Errorf("%w: field too long", ErrInvalidInput)
	case in.Amount <= 0:
		return fmt.Errorf("%w: amount must be positive", ErrInvalidInput)
	case in.Amount > max:
		return ErrAmountTooLarge
	case len(in.Currency) != 3 || in.Currency != strings.ToUpper(in.Currency):
		return fmt.Errorf("%w: currency", ErrInvalidInput)
	case !phone.Valid(in.Phone):
		return fmt.Errorf("%w: phone must be E.164", ErrInvalidInput)
	}
	return nil
}

// Create starts a payout. It is idempotent per merchant and key: a repeat
// returns the original payout, never holds funds twice and never asks the
// operator to pay twice, even if an earlier attempt stopped half way.
//
// If the merchant lacks the funds the payout is stored as rejected and
// ErrInsufficientFunds is returned, again on every repeat.
func (s *Service) Create(ctx context.Context, in CreateInput) (Payout, error) {
	if err := in.validate(s.cfg.MaxAmount); err != nil {
		return Payout{}, err
	}
	adapter, err := s.d.Router.Route(in.Phone)
	if errors.Is(err, router.ErrNoRoute) {
		return Payout{}, ErrUnsupportedNumber
	}
	if err != nil {
		return Payout{}, err
	}
	fee, err := s.feeFor(ctx, in)
	if err != nil {
		return Payout{}, err
	}
	p, created, err := s.d.Repo.Create(ctx, s.newPayout(in, fee, adapter.Code()))
	if err != nil {
		return Payout{}, err
	}
	if !created {
		if done, err := repeatOutcome(p); done {
			return p, err
		}
	}
	return s.advance(ctx, p, adapter, in.Phone)
}

// repeatOutcome decides what a repeated request returns. A payout that is
// past its first steps is returned as it stands; a rejected one is rejected
// again; one still starting is carried on by the caller.
func repeatOutcome(p Payout) (done bool, err error) {
	switch p.Status {
	case StatusRejected:
		return true, ErrInsufficientFunds
	case StatusCreated, StatusHeld:
		return false, nil
	}
	return true, nil
}

func (s *Service) feeFor(ctx context.Context, in CreateInput) (int64, error) {
	bps, err := s.d.Terms.PayoutFeeBPS(ctx, in.MerchantID)
	if err != nil {
		return 0, fmt.Errorf("merchant terms: %w", err)
	}
	fee, err := money.Fee(in.Amount, bps)
	if err != nil {
		return 0, fmt.Errorf("%w: fee", ErrInvalidInput)
	}
	return fee, nil
}

func (s *Service) newPayout(in CreateInput, fee int64, operator string) NewPayout {
	now := s.now()
	grace := now.Add(s.cfg.CreatedGrace)
	return NewPayout{
		Phone: in.Phone,
		Payout: Payout{
			ID: ids.New(idPrefix), MerchantID: in.MerchantID, IdempotencyKey: in.IdempotencyKey,
			MerchantReference: in.Reference, Status: StatusCreated, Amount: in.Amount,
			Currency: in.Currency, Fee: fee, PhoneMasked: phone.Mask(in.Phone),
			Description: in.Description, Operator: operator, NextCheckAt: &grace,
			CreatedAt: now, UpdatedAt: now,
		},
	}
}

// hold reserves the funds. An empty or short balance is not an error of the
// system: it is reported as ErrInsufficientFunds.
func (s *Service) hold(ctx context.Context, p Payout) error {
	if err := s.ensureAccounts(ctx, p); err != nil {
		return err
	}
	err := s.post(ctx, p.holdJournal())
	if errors.Is(err, ledger.ErrInsufficientFunds) {
		return ErrInsufficientFunds
	}
	return err
}
