package collections

import "math"

const (
	basisPoints = 10_000
	halfUp      = basisPoints / 2
)

// ComputeFee returns amount * bps / 10000, rounded half up, in the smallest
// currency unit. bps is a percentage in hundredths of a percent, so 150 is
// 1.5 percent. The result never exceeds amount.
func ComputeFee(amount int64, bps int) (int64, error) {
	if amount <= 0 || bps < 0 || bps > basisPoints {
		return 0, ErrInvalidInput
	}
	if amount > (math.MaxInt64-halfUp)/int64(bps+1) {
		return 0, ErrFeeOverflow
	}
	return (amount*int64(bps) + halfUp) / basisPoints, nil
}
