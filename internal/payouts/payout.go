package payouts

import "time"

// Payout is money sent to a phone number. It never holds the recipient's
// phone number, only a masked form for display.
type Payout struct {
	ID                string
	MerchantID        string
	IdempotencyKey    string
	MerchantReference string
	Status            Status
	Amount            int64
	Currency          string
	Fee               int64
	PhoneMasked       string
	PhoneIndex        []byte
	Description       string
	Operator          string
	OperatorRef       string
	FailureCode       string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	CompletedAt       *time.Time
	NextCheckAt       *time.Time
}

// Total is what leaves the merchant's balance: the amount plus our fee.
func (p Payout) Total() int64 { return p.Amount + p.Fee }

// Event is one entry in a payout's audit trail.
type Event struct {
	Type   string
	From   Status
	To     Status
	Detail string // short, never personal data
	At     time.Time
}

// Event types.
const (
	EventHeld           = "held"
	EventInitiated      = "initiated"
	EventConfirmed      = "confirmed"
	EventRejected       = "rejected"
	EventUnknown        = "outcome_unknown"
	EventAbandoned      = "abandoned"
	EventAmountMismatch = "amount_mismatch"
	EventContradiction  = "contradiction"
	EventStalePending   = "stale_pending"
)

// Failure codes recorded on payouts.
const (
	CodeInsufficientFunds = "insufficient_funds"
	CodeAmountMismatch    = "amount_mismatch" // held for a person
	codeRejected          = "rejected"
	codeNeverReceived     = "operator_never_received"
	codeAbandoned         = "abandoned"
	codeExpired           = "expired"
)

// NewPayout is a payout to insert, with the plaintext phone number that the
// store seals before it is written.
type NewPayout struct {
	Payout
	Phone string
}

// Outbox event types, one per announced final status.
const (
	OutboxSucceeded = "payout.succeeded"
	OutboxFailed    = "payout.failed"
)

// OutboxType returns the event announcing status s, if it is announced.
func OutboxType(s Status) (string, bool) {
	switch s {
	case StatusSucceeded:
		return OutboxSucceeded, true
	case StatusFailed:
		return OutboxFailed, true
	}
	return "", false
}

// OutboxEntry announces that a payout reached a final status. It is written
// in the same transaction as the status change.
type OutboxEntry struct {
	ID        int64
	PayoutID  string
	EventType string
	CreatedAt time.Time
}
