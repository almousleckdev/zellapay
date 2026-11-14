package collections

import "time"

// Collection is a request for money from a customer's phone. It never holds
// the customer's phone number, only a masked form for display.
type Collection struct {
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

// Event is one entry in a collection's audit trail.
type Event struct {
	Type   string
	From   Status
	To     Status
	Detail string // short, never personal data
	At     time.Time
}

// Event types.
const (
	EventInitiated      = "initiated"
	EventConfirmed      = "confirmed"
	EventRejected       = "rejected"
	EventUnknown        = "outcome_unknown"
	EventAmountMismatch = "amount_mismatch"
	EventContradiction  = "contradiction"
	EventStalePending   = "stale_pending"
)

// CodeAmountMismatch marks a collection held for a person because the
// operator's callback disagreed with the request.
const CodeAmountMismatch = "amount_mismatch"

// NewCollection is a collection to insert, with the plaintext phone number
// that the store seals before it is written.
type NewCollection struct {
	Collection
	Phone string
}
