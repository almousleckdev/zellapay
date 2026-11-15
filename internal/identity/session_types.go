package identity

import "time"

// Source describes where a request came from. It is kept with a session so a
// person can recognise their own devices, and never used to authorise.
type Source struct {
	IP        string
	UserAgent string
}

// Session is one signed-in browser. The token that proves it is never
// stored, only its hash.
type Session struct {
	ID         string
	UserID     string
	TokenHash  [32]byte
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time // the absolute limit, whatever the activity
	MFAAt      *time.Time
	Source     Source
	RevokedAt  *time.Time
}

// MFAFresh reports whether the second factor was proven within the last
// window. The access module uses it to demand step-up before sensitive actions.
func (s Session) MFAFresh(now time.Time, window time.Duration) bool {
	return s.MFAAt != nil && now.Sub(*s.MFAAt) <= window
}

// MFAState tells a caller what a fresh session still has to do.
type MFAState string

// What a session needs next.
const (
	// MFADone means the second factor has been proven for this session.
	MFADone MFAState = "done"
	// MFARequired means the user has MFA and must enter a code.
	MFARequired MFAState = "required"
	// MFAEnrolRequired means the user must set MFA up before anything else.
	MFAEnrolRequired MFAState = "enrol_required"
)

// Principal is who a valid session belongs to.
type Principal struct {
	Session Session
	User    User
	MFA     MFAState
}

// SessionConfig sets how long sessions live.
type SessionConfig struct {
	Idle     time.Duration
	Absolute time.Duration
	// TouchEvery limits how often activity is written back to the database.
	TouchEvery time.Duration
}

// MerchantSessionConfig returns the lifetimes from the design.
func MerchantSessionConfig() SessionConfig {
	return SessionConfig{
		Idle: 30 * time.Minute, Absolute: 12 * time.Hour, TouchEvery: time.Minute,
	}
}
