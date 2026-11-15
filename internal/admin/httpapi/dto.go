package httpapi

import (
	"time"

	"github.com/zellapay/zellapay/internal/merchants"
)

type merchantJSON struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Status          string    `json:"status"`
	KYCStatus       string    `json:"kyc_status"`
	DefaultCurrency string    `json:"default_currency"`
	CreatedAt       time.Time `json:"created_at"`
}

type detailJSON struct {
	merchantJSON
	FeeBPS       int `json:"fee_bps"`
	PayoutFeeBPS int `json:"payout_fee_bps"`
}

type listResponse struct {
	Data       []merchantJSON `json:"data"`
	NextCursor *string        `json:"next_cursor"`
}

type reasonRequest struct {
	Reason string `json:"reason"`
}

func merchantView(m merchants.Merchant) merchantJSON {
	return merchantJSON{
		ID: m.ID, Name: m.Name, Status: string(m.Status), KYCStatus: m.KYCStatus,
		DefaultCurrency: m.DefaultCurrency, CreatedAt: m.CreatedAt,
	}
}
