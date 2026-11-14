// Package money holds arithmetic on amounts in the smallest currency unit.
package money

import (
	"errors"
	"math"
)

const (
	basisPoints = 10_000
	halfUp      = basisPoints / 2
)

// Errors returned by Fee.
var (
	ErrInvalid  = errors.New("money: invalid amount or rate")
	ErrOverflow = errors.New("money: calculation overflow")
)

// Fee returns amount * bps / 10000, rounded half up, in the smallest
// currency unit. bps is a percentage in hundredths of a percent, so 150 is
// 1.5 percent. The result never exceeds amount.
func Fee(amount int64, bps int) (int64, error) {
	if amount <= 0 || bps < 0 || bps > basisPoints {
		return 0, ErrInvalid
	}
	if amount > (math.MaxInt64-halfUp)/int64(bps+1) {
		return 0, ErrOverflow
	}
	return (amount*int64(bps) + halfUp) / basisPoints, nil
}
