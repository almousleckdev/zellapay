package ledger

import "errors"

var (
	// ErrUnknownAccount means a journal references a missing account.
	ErrUnknownAccount = errors.New("ledger: unknown account")
	// ErrInsufficientFunds means a posting would overdraw an account.
	ErrInsufficientFunds = errors.New("ledger: insufficient funds")
	// ErrIdempotencyConflict means a key was reused with a different journal.
	ErrIdempotencyConflict = errors.New("ledger: idempotency key reused")
	// ErrInvalidJournal means a journal header field is missing.
	ErrInvalidJournal = errors.New("ledger: invalid journal")
)
