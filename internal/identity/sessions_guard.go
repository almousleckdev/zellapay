package identity

import (
	"context"
	"errors"
)

// guard refuses an attempt that is being throttled. A throttle that cannot be
// reached refuses too: guessing must never become free because Redis is down.
func (s *Sessions) guard(ctx context.Context, a Attempt) error {
	wait, err := s.d.Throttle.Wait(ctx, a)
	if err != nil {
		return err
	}
	if wait > 0 {
		return &ThrottledError{RetryAfter: wait}
	}
	return nil
}

// failed counts a wrong answer and returns the error to show. Only a wrong
// answer is counted: an outage or a malformed request is not the user's fault.
func (s *Sessions) failed(ctx context.Context, a Attempt, cause error) error {
	if !errors.Is(cause, ErrInvalidCredentials) && !errors.Is(cause, ErrInvalidCode) {
		return cause
	}
	if err := s.d.Throttle.Failed(ctx, a); err != nil {
		s.d.Log.Error("throttle: record failure", "realm", a.Realm, "error", err)
	}
	return cause
}

func (s *Sessions) succeeded(ctx context.Context, a Attempt) {
	if err := s.d.Throttle.Succeeded(ctx, a); err != nil {
		s.d.Log.Warn("throttle: clear failures", "realm", a.Realm, "error", err)
	}
}
