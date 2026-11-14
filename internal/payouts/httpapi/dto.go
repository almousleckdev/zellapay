// Package httpapi exposes payouts over HTTP according to api/openapi.yaml.
// It translates between JSON and the domain and contains no business rules.
package httpapi

import (
	"time"

	"github.com/zellapay/zellapay/internal/payouts"
)

// createRequest is the body of POST /v1/payouts and one item of a batch.
type createRequest struct {
	Amount            int64  `json:"amount"`
	Currency          string `json:"currency"`
	RecipientPhone    string `json:"recipient_phone"`
	MerchantReference string `json:"merchant_reference"`
	Description       string `json:"description"`
}

type batchRequest struct {
	Payouts []createRequest `json:"payouts"`
}

// payoutResponse is the public form of a payout.
type payoutResponse struct {
	ID                string     `json:"id"`
	Status            string     `json:"status"`
	Amount            int64      `json:"amount"`
	Currency          string     `json:"currency"`
	Fee               int64      `json:"fee"`
	RecipientPhone    string     `json:"recipient_phone_masked"`
	MerchantReference string     `json:"merchant_reference"`
	Operator          string     `json:"operator,omitempty"`
	FailureCode       *string    `json:"failure_code"`
	Description       string     `json:"description,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	CompletedAt       *time.Time `json:"completed_at"`
}

type listResponse struct {
	Data       []payoutResponse `json:"data"`
	HasMore    bool             `json:"has_more"`
	NextCursor *string          `json:"next_cursor"`
}

func toResponse(p payouts.Payout) payoutResponse {
	r := payoutResponse{
		ID: p.ID, Status: publicStatus(p.Status), Amount: p.Amount, Currency: p.Currency,
		Fee: p.Fee, RecipientPhone: p.PhoneMasked, MerchantReference: p.MerchantReference,
		Operator: p.Operator, Description: p.Description,
		CreatedAt: p.CreatedAt.UTC(), CompletedAt: utcPtr(p.CompletedAt),
	}
	if r.Status == "failed" {
		code := publicFailure(p.FailureCode)
		r.FailureCode = &code
	}
	return r
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func toListResponse(page payouts.Page) listResponse {
	out := listResponse{Data: make([]payoutResponse, len(page.Items)), HasMore: page.HasMore}
	for i, p := range page.Items {
		out.Data[i] = toResponse(p)
	}
	if page.HasMore {
		cursor := page.NextCursor
		out.NextCursor = &cursor
	}
	return out
}
