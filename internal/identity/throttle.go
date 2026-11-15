package identity

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrThrottled means too many recent attempts. Callers read RetryAfter from
// the ThrottledError that wraps it.
var ErrThrottled = errors.New("identity: too many attempts")

// ThrottledError carries how long the caller must wait.
type ThrottledError struct{ RetryAfter time.Duration }

func (e *ThrottledError) Error() string {
	return fmt.Sprintf("%v: retry in %s", ErrThrottled, e.RetryAfter.Round(time.Second))
}

// Is lets errors.Is(err, ErrThrottled) match.
func (e *ThrottledError) Is(target error) bool { return target == ErrThrottled }

// Realm separates the counters of different checks.
type Realm string

// The checks that are throttled.
const (
	RealmPassword Realm = "password"
	RealmMFA      Realm = "mfa"
)

// Attempt names one try: whose account it targets and where it came from.
// Account is an email for passwords and a user ID for MFA codes.
type Attempt struct {
	Realm   Realm
	Account string
	IP      string
}

// Throttle slows down guessing. Implementations keep counters outside the
// database, never store the account or address in the clear, and treat an
// unknown account exactly like a known one.
type Throttle interface {
	// Wait returns how long the caller must wait before trying, or zero.
	Wait(ctx context.Context, a Attempt) (time.Duration, error)
	// Failed records a failed try.
	Failed(ctx context.Context, a Attempt) error
	// Succeeded clears the account's failures after a correct answer.
	Succeeded(ctx context.Context, a Attempt) error
}
