// Package ledger implements the double-entry ledger, the source of truth
// for all money.
package ledger

import (
	"errors"
	"fmt"
)

// Currency is an ISO 4217 code such as "XAF".
type Currency string

// Side is the direction of an entry.
type Side int8

// Sides of a ledger entry.
const (
	Debit Side = iota + 1
	Credit
)

// Entry is one line of a journal. Amount is in the smallest currency unit and
// must be positive; direction is carried by Side.
type Entry struct {
	Account  string
	Side     Side
	Amount   int64
	Currency Currency
}

func (e Entry) wellFormed() bool {
	validSide := e.Side == Debit || e.Side == Credit
	return e.Account != "" && e.Currency != "" && e.Amount > 0 && validSide
}

// Validation errors returned by Validate.
var (
	ErrEmptyJournal   = errors.New("ledger: journal has fewer than two entries")
	ErrInvalidEntry   = errors.New("ledger: invalid entry")
	ErrUnbalanced     = errors.New("ledger: entries do not balance")
	ErrAmountOverflow = errors.New("ledger: amount overflow")
)

// Validate checks that a journal is well formed and balances per currency:
// total debits equal total credits.
func Validate(entries []Entry) error {
	if len(entries) < 2 {
		return ErrEmptyJournal
	}
	net := map[Currency]int64{}
	for i, e := range entries {
		if !e.wellFormed() {
			return fmt.Errorf("%w: entry %d", ErrInvalidEntry, i)
		}
		sum, ok := addSigned(net[e.Currency], e)
		if !ok {
			return ErrAmountOverflow
		}
		net[e.Currency] = sum
	}
	return checkBalanced(net)
}

// addSigned adds e to a running net (debits positive, credits negative)
// and reports false if the addition would overflow int64.
func addSigned(net int64, e Entry) (int64, bool) {
	delta := e.Amount
	if e.Side == Credit {
		delta = -delta
	}
	sum := net + delta
	overflow := (delta > 0 && sum < net) || (delta < 0 && sum > net)
	return sum, !overflow
}

func checkBalanced(net map[Currency]int64) error {
	for c, v := range net {
		if v != 0 {
			return fmt.Errorf("%w: %s off by %d", ErrUnbalanced, c, v)
		}
	}
	return nil
}

// String returns the storage name of the side.
func (s Side) String() string {
	if s == Debit {
		return "debit"
	}
	return "credit"
}
