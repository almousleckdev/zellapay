package payouts

import "errors"

// Errors returned by the module.
var (
	ErrInvalidInput        = errors.New("payouts: invalid input")
	ErrNotFound            = errors.New("payouts: not found")
	ErrUnsupportedNumber   = errors.New("payouts: no operator serves this number")
	ErrInsufficientFunds   = errors.New("payouts: insufficient funds")
	ErrAmountTooLarge      = errors.New("payouts: amount exceeds the limit")
	ErrIdempotencyConflict = errors.New("payouts: idempotency key reused")
	ErrInvalidTransition   = errors.New("payouts: invalid state transition")
	ErrNoChange            = errors.New("payouts: nothing to change")
	ErrUnknownOperator     = errors.New("payouts: unknown operator")
)
