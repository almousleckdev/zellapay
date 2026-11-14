package collections

import "errors"

// Errors returned by the module.
var (
	ErrInvalidInput        = errors.New("collections: invalid input")
	ErrNotFound            = errors.New("collections: not found")
	ErrUnsupportedNumber   = errors.New("collections: no operator serves this number")
	ErrIdempotencyConflict = errors.New("collections: idempotency key reused")
	ErrInvalidTransition   = errors.New("collections: invalid state transition")
	ErrNoChange            = errors.New("collections: nothing to change")
	ErrUnknownOperator     = errors.New("collections: unknown operator")
	ErrUnknownReference    = errors.New("collections: unknown reference")
	ErrFeeOverflow         = errors.New("collections: fee calculation overflow")
)
