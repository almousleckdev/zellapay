// Package httpapi exposes collections over HTTP according to
// api/openapi.yaml. It translates between JSON and the domain and never
// contains business rules.
package httpapi

import (
	"time"

	"github.com/zellapay/zellapay/internal/collections"
)

// createRequest is the body of POST /v1/collections.
type createRequest struct {
	Amount            int64  `json:"amount"`
	Currency          string `json:"currency"`
	CustomerPhone     string `json:"customer_phone"`
	MerchantReference string `json:"merchant_reference"`
	Description       string `json:"description"`
}

// collectionResponse is the public form of a collection.
type collectionResponse struct {
	ID                string     `json:"id"`
	Status            string     `json:"status"`
	Amount            int64      `json:"amount"`
	Currency          string     `json:"currency"`
	Fee               *int64     `json:"fee,omitempty"`
	CustomerPhone     string     `json:"customer_phone_masked"`
	MerchantReference string     `json:"merchant_reference"`
	Operator          string     `json:"operator,omitempty"`
	FailureCode       *string    `json:"failure_code"`
	Description       string     `json:"description,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	CompletedAt       *time.Time `json:"completed_at"`
}

type listResponse struct {
	Data       []collectionResponse `json:"data"`
	HasMore    bool                 `json:"has_more"`
	NextCursor *string              `json:"next_cursor"`
}

func toResponse(c collections.Collection) collectionResponse {
	r := collectionResponse{
		ID:                c.ID,
		Status:            string(publicStatus(c.Status)),
		Amount:            c.Amount,
		Currency:          c.Currency,
		CustomerPhone:     c.PhoneMasked,
		MerchantReference: c.MerchantReference,
		Operator:          c.Operator,
		Description:       c.Description,
		CreatedAt:         c.CreatedAt.UTC(),
		CompletedAt:       utcPtr(c.CompletedAt),
	}
	if c.Status == collections.StatusSucceeded {
		fee := c.Fee
		r.Fee = &fee
	}
	if c.Status == collections.StatusFailed {
		code := publicFailure(c.FailureCode)
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
