package pgstore

import (
	"github.com/zellapay/zellapay/internal/payouts"
	"github.com/zellapay/zellapay/internal/payouts/pgstore/sqlcgen"
	"github.com/zellapay/zellapay/internal/platform/pgconv"
)

var (
	ts      = pgconv.Timestamptz
	optTS   = pgconv.OptTimestamptz
	timePtr = pgconv.TimePtr
)

func insertParams(nc payouts.NewPayout, sealedPhone string) sqlcgen.InsertPayoutParams {
	return sqlcgen.InsertPayoutParams{
		ID:                   nc.ID,
		MerchantID:           nc.MerchantID,
		IdempotencyKey:       nc.IdempotencyKey,
		MerchantReference:    nc.MerchantReference,
		Amount:               nc.Amount,
		Currency:             nc.Currency,
		FeeAmount:            nc.Fee,
		RecipientPhoneEnc:    sealedPhone,
		RecipientPhoneIdx:    nc.PhoneIndex,
		RecipientPhoneMasked: nc.PhoneMasked,
		Description:          nc.Description,
		Operator:             nc.Operator,
		Now:                  ts(nc.CreatedAt),
		NextCheckAt:          optTS(nc.NextCheckAt),
	}
}

func fromRow(r sqlcgen.GetPayoutRow) payouts.Payout {
	return payouts.Payout{
		ID:                r.ID,
		MerchantID:        r.MerchantID,
		IdempotencyKey:    r.IdempotencyKey,
		MerchantReference: r.MerchantReference,
		Status:            payouts.Status(r.Status),
		Amount:            r.Amount,
		Currency:          r.Currency,
		Fee:               r.FeeAmount,
		PhoneMasked:       r.RecipientPhoneMasked,
		PhoneIndex:        r.RecipientPhoneIdx,
		Description:       r.Description,
		Operator:          r.Operator,
		OperatorRef:       r.OperatorRef,
		FailureCode:       r.FailureCode,
		CreatedAt:         r.CreatedAt.Time,
		UpdatedAt:         r.UpdatedAt.Time,
		CompletedAt:       timePtr(r.CompletedAt),
		NextCheckAt:       timePtr(r.NextCheckAt),
	}
}

func updateParams(c payouts.Payout) sqlcgen.UpdatePayoutParams {
	return sqlcgen.UpdatePayoutParams{
		ID:          c.ID,
		Status:      string(c.Status),
		Operator:    c.Operator,
		OperatorRef: c.OperatorRef,
		FailureCode: c.FailureCode,
		UpdatedAt:   ts(c.UpdatedAt),
		CompletedAt: optTS(c.CompletedAt),
		NextCheckAt: optTS(c.NextCheckAt),
	}
}

func eventParams(id string, ev payouts.Event) sqlcgen.InsertPayoutEventParams {
	return sqlcgen.InsertPayoutEventParams{
		PayoutID:   id,
		Type:       ev.Type,
		FromStatus: string(ev.From),
		ToStatus:   string(ev.To),
		Detail:     ev.Detail,
		CreatedAt:  ts(ev.At),
	}
}
