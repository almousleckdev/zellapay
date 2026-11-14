// Package operators defines the contract that every mobile money
// operator adapter implements.
package operators

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// State is the normalized outcome of an operator transaction.
type State int8

// Normalized operator transaction states.
const (
	StateUnknown State = iota // outcome not known; never treat as failure
	StatePending
	StateSucceeded
	StateFailed
	// StateExpired means the operator gave up waiting for the customer.
	StateExpired
)

// Errors returned by adapters.
var (
	// ErrUnknownOutcome means the call may or may not have taken effect
	// (timeout, connection reset, malformed reply). Callers must not
	// assume failure.
	ErrUnknownOutcome = errors.New("operators: unknown outcome")
	// ErrNotFound means the operator has no record of the reference. For
	// a request that timed out, it means the request never arrived.
	ErrNotFound = errors.New("operators: reference not found")
	// ErrDuplicateReference means the reference was already used for a
	// different request.
	ErrDuplicateReference = errors.New("operators: duplicate reference")
	// ErrRejected means the operator definitively refused the request, so
	// nothing happened and it is safe to treat the request as failed.
	ErrRejected = errors.New("operators: request rejected")
	// ErrInvalidCallback means a callback failed verification or parsing.
	ErrInvalidCallback = errors.New("operators: invalid callback")
)

// Ref identifies a transaction at the operator.
type Ref struct {
	OperatorRef string // operator's identifier, if one was returned
	OurRef      string // our reference, sent to the operator
}

// CollectionRequest asks a customer's phone to approve a payment.
type CollectionRequest struct {
	Reference string
	Phone     string // E.164
	Amount    int64  // smallest unit
	Currency  string
	Narration string
}

// PayoutRequest sends money to a phone number.
type PayoutRequest struct {
	Reference string
	Phone     string
	Amount    int64
	Currency  string
	Narration string
}

// Status is the normalized result of a status query.
type Status struct {
	State   State
	RawCode string
	RawBody []byte // kept for audit, secrets redacted
	Fee     *int64
}

// CallbackEvent is a verified, parsed operator callback. Amount and
// Currency are what the operator reports; the caller must compare them
// with the original request before trusting the event.
type CallbackEvent struct {
	Ref      Ref
	Status   Status
	Amount   int64
	Currency string
}

// Health describes an adapter's current condition.
type Health struct {
	OK        bool
	CheckedAt time.Time
	Detail    string
}

// Adapter is implemented once per operator. Adapters apply deadlines from ctx,
// perform no internal retries, and return ErrUnknownOutcome on
// ambiguous failures.
type Adapter interface {
	Code() string // "airtel_money", "moov_money", "simulator"
	InitiateCollection(ctx context.Context, r CollectionRequest) (Ref, error)
	InitiatePayout(ctx context.Context, r PayoutRequest) (Ref, error)
	Status(ctx context.Context, ref Ref) (Status, error)
	ParseCallback(ctx context.Context, req *http.Request) (CallbackEvent, error)
	Health(ctx context.Context) Health
}
