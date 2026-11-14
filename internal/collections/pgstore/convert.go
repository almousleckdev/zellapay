package pgstore

import (
	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/collections/pgstore/sqlcgen"
	"github.com/zellapay/zellapay/internal/platform/pgconv"
)

var (
	ts      = pgconv.Timestamptz
	optTS   = pgconv.OptTimestamptz
	timePtr = pgconv.TimePtr
)

func insertParams(nc collections.NewCollection, sealedPhone string) sqlcgen.InsertCollectionParams {
	return sqlcgen.InsertCollectionParams{
		ID:                  nc.ID,
		MerchantID:          nc.MerchantID,
		IdempotencyKey:      nc.IdempotencyKey,
		MerchantReference:   nc.MerchantReference,
		Amount:              nc.Amount,
		Currency:            nc.Currency,
		FeeAmount:           nc.Fee,
		CustomerPhoneEnc:    sealedPhone,
		CustomerPhoneIdx:    nc.PhoneIndex,
		CustomerPhoneMasked: nc.PhoneMasked,
		Description:         nc.Description,
		Operator:            nc.Operator,
		Now:                 ts(nc.CreatedAt),
		NextCheckAt:         optTS(nc.NextCheckAt),
	}
}

func fromRow(r sqlcgen.GetCollectionRow) collections.Collection {
	return collections.Collection{
		ID:                r.ID,
		MerchantID:        r.MerchantID,
		IdempotencyKey:    r.IdempotencyKey,
		MerchantReference: r.MerchantReference,
		Status:            collections.Status(r.Status),
		Amount:            r.Amount,
		Currency:          r.Currency,
		Fee:               r.FeeAmount,
		PhoneMasked:       r.CustomerPhoneMasked,
		PhoneIndex:        r.CustomerPhoneIdx,
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

func updateParams(c collections.Collection) sqlcgen.UpdateCollectionParams {
	return sqlcgen.UpdateCollectionParams{
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

func eventParams(id string, ev collections.Event) sqlcgen.InsertCollectionEventParams {
	return sqlcgen.InsertCollectionEventParams{
		CollectionID: id,
		Type:         ev.Type,
		FromStatus:   string(ev.From),
		ToStatus:     string(ev.To),
		Detail:       ev.Detail,
		CreatedAt:    ts(ev.At),
	}
}
