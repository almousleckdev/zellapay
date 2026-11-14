package notify

import "time"

const (
	backoffBase = 30 * time.Second
	backoffCap  = time.Hour
	// RetryHorizon is how long a message keeps being retried.
	RetryHorizon = 24 * time.Hour
)

// Backoff returns how long to wait after the given number of failed
// attempts: 30s, 1m, 2m, 4m and so on, doubling up to one hour.
func Backoff(failedAttempts int) time.Duration {
	if failedAttempts < 1 {
		failedAttempts = 1
	}
	d := backoffBase
	for i := 1; i < failedAttempts && d < backoffCap; i++ {
		d *= 2
	}
	return min(d, backoffCap)
}
