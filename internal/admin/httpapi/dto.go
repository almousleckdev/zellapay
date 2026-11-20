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

type createRequest struct {
	Name     string `json:"name"`
	Currency string `json:"currency"`
	Reason   string `json:"reason"`
}

type inviteRequest struct {
	Email  string `json:"email"`
	Role   string `json:"role"`
	Locale string `json:"locale"`
	Reason string `json:"reason"`
}

type invitationJSON struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
}

type dayJSON struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type overviewResponse struct {
	Total            int            `json:"total"`
	ByStatus         map[string]int `json:"by_status"`
	CreatedByDay     []dayJSON      `json:"created_by_day"`
	PendingApprovals int            `json:"pending_approvals"`
}

type requestJSON struct {
	ID             string     `json:"id"`
	MerchantID     string     `json:"merchant_id"`
	MerchantName   string     `json:"merchant_name,omitempty"`
	Kind           string     `json:"kind"`
	FeeBPS         *int       `json:"fee_bps"`
	PayoutFeeBPS   *int       `json:"payout_fee_bps"`
	Reason         string     `json:"reason"`
	RequestedBy    string     `json:"requested_by"`
	RequestedByYou bool       `json:"requested_by_you"`
	RequestedAt    time.Time  `json:"requested_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	Status         string     `json:"status"`
	DecidedBy      *string    `json:"decided_by"`
	DecidedAt      *time.Time `json:"decided_at"`
	DecisionReason string     `json:"decision_reason"`
}

type requestsResponse struct {
	Data []requestJSON `json:"data"`
}

type feesRequest struct {
	FeeBPS       int    `json:"fee_bps"`
	PayoutFeeBPS int    `json:"payout_fee_bps"`
	Reason       string `json:"reason"`
}

type activityJSON struct {
	Action string            `json:"action"`
	Actor  string            `json:"actor"`
	Reason string            `json:"reason"`
	At     time.Time         `json:"at"`
	Change map[string]string `json:"change"`
}

type activityResponse struct {
	Data []activityJSON `json:"data"`
}

type operatorJSON struct {
	Code     string   `json:"code"`
	Prefixes []string `json:"prefixes"`
}

type operatorsResponse struct {
	Data []operatorJSON `json:"data"`
}
