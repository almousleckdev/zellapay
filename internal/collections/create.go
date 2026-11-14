package collections

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zellapay/zellapay/internal/operators/router"
	"github.com/zellapay/zellapay/internal/platform/ids"
	"github.com/zellapay/zellapay/internal/platform/money"
	"github.com/zellapay/zellapay/internal/platform/phone"
)

// CreateInput is a request to collect money.
type CreateInput struct {
	MerchantID     string
	IdempotencyKey string
	Reference      string
	Phone          string // E.164
	Currency       string
	Description    string
	Amount         int64
}

func (in CreateInput) validate() error {
	switch {
	case in.MerchantID == "", in.IdempotencyKey == "", in.Reference == "":
		return fmt.Errorf("%w: missing field", ErrInvalidInput)
	case len(in.Reference) > 128 || len(in.Description) > 200:
		return fmt.Errorf("%w: field too long", ErrInvalidInput)
	case in.Amount <= 0:
		return fmt.Errorf("%w: amount must be positive", ErrInvalidInput)
	case len(in.Currency) != 3 || in.Currency != strings.ToUpper(in.Currency):
		return fmt.Errorf("%w: currency", ErrInvalidInput)
	case !phone.Valid(in.Phone):
		return fmt.Errorf("%w: phone must be E.164", ErrInvalidInput)
	}
	return nil
}

// Create starts a collection. It is idempotent per merchant and key: a
// repeat returns the original collection and never asks the operator twice,
// even if an earlier attempt stopped half way.
func (s *Service) Create(ctx context.Context, in CreateInput) (Collection, error) {
	if err := in.validate(); err != nil {
		return Collection{}, err
	}
	adapter, err := s.d.Router.Route(in.Phone)
	if errors.Is(err, router.ErrNoRoute) {
		return Collection{}, ErrUnsupportedNumber
	}
	if err != nil {
		return Collection{}, err
	}
	fee, err := s.feeFor(ctx, in)
	if err != nil {
		return Collection{}, err
	}
	c, created, err := s.d.Repo.Create(ctx, s.newCollection(in, fee, adapter.Code()))
	if err != nil {
		return Collection{}, err
	}
	if !created && c.Status != StatusCreated {
		return c, nil
	}
	return s.initiate(ctx, c, adapter, in.Phone)
}

func (s *Service) feeFor(ctx context.Context, in CreateInput) (int64, error) {
	bps, err := s.d.Terms.FeeBPS(ctx, in.MerchantID)
	if err != nil {
		return 0, fmt.Errorf("merchant terms: %w", err)
	}
	fee, err := money.Fee(in.Amount, bps)
	if err != nil {
		return 0, fmt.Errorf("%w: fee", ErrInvalidInput)
	}
	return fee, nil
}

func (s *Service) newCollection(in CreateInput, fee int64, operator string) NewCollection {
	now := s.now()
	grace := now.Add(s.cfg.CreatedGrace)
	return NewCollection{
		Phone: in.Phone,
		Collection: Collection{
			ID:                ids.New(idPrefix),
			MerchantID:        in.MerchantID,
			IdempotencyKey:    in.IdempotencyKey,
			MerchantReference: in.Reference,
			Status:            StatusCreated,
			Amount:            in.Amount,
			Currency:          in.Currency,
			Fee:               fee,
			PhoneMasked:       phone.Mask(in.Phone),
			Description:       in.Description,
			Operator:          operator,
			NextCheckAt:       &grace,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	}
}
