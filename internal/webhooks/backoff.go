package webhooks

import "time"

const (
	backoffBase = 30 * time.Second
	backoffCap  = 6 * time.Hour
	// RetryHorizon is how long a delivery keeps being retried.
	RetryHorizon = 72 * time.Hour
)

// Backoff returns how long to wait after the given number of failed
// attempts: 30s, 1m, 2m, 4m and so on, doubling up to six hours.
func Backoff(failedAttempts int) time.Duration {
	if failedAttempts < 1 {
		failedAttempts = 1
	}
	d := backoffBase
	for i := 1; i < failedAttempts && d < backoffCap; i++ {
		d *= 2
	}
	if d > backoffCap {
		return backoffCap
	}
	return d
}
