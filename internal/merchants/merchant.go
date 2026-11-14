// Package merchants manages merchants, their API keys and the
// authentication of merchant requests.
package merchants

import (
	"errors"
	"time"
)

// Status is a merchant's lifecycle state.
type Status string

// Merchant lifecycle states.
const (
	StatusPending   Status = "pending"
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
)

// Merchant is a business that integrates with ZellaPay.
type Merchant struct {
	ID              string
	Name            string
	Status          Status
	KYCStatus       string
	DefaultCurrency string
	CreatedAt       time.Time
}

// Errors returned by the merchants module.
var (
	ErrNotFound         = errors.New("merchants: not found")
	ErrInvalidInput     = errors.New("merchants: invalid input")
	ErrInvalidKey       = errors.New("merchants: invalid api key")
	ErrWrongMode        = errors.New("merchants: key mode does not match environment")
	ErrMerchantInactive = errors.New("merchants: merchant is not active")
	ErrIPNotAllowed     = errors.New("merchants: ip address not allowed")
	ErrScopeMissing     = errors.New("merchants: scope missing")
)
