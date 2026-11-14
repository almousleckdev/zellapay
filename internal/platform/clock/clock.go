// Package clock abstracts time so domain code stays deterministic in tests.
package clock

import "time"

// Clock reports the current time.
type Clock interface {
	Now() time.Time
}

// System is the real wall clock, in UTC.
type System struct{}

// Now returns the current UTC time.
func (System) Now() time.Time { return time.Now().UTC() }
