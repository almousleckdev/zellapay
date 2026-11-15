// Package staff manages ZellaPay's own people: how they sign in with a
// password and a passkey, and their sessions. They are kept apart from
// merchant users in every way: tables, sessions, cookies and credentials.
package staff

import "time"

// Role is what a staff member does. The admin module turns roles into
// permissions.
type Role string

// Staff roles.
const (
	RoleSupport     Role = "support"
	RoleRiskAnalyst Role = "risk_analyst"
	RoleFinance     Role = "finance"
	RoleAdmin       Role = "admin"
	RoleSuperAdmin  Role = "super_admin"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	switch r {
	case RoleSupport, RoleRiskAnalyst, RoleFinance, RoleAdmin, RoleSuperAdmin:
		return true
	}
	return false
}

// Status is the state of a staff account.
type Status string

// Account states.
const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

// User is a member of staff.
type User struct {
	ID                string
	Email             string
	Name              string
	Role              Role
	Status            Status
	PasswordHash      string
	PasswordChangedAt time.Time
	CreatedBy         string
	CreatedAt         time.Time
}

// Passkey is a registered authenticator. Data is the library's serialised
// credential, which holds the public key and the signature counter.
type Passkey struct {
	ID           string
	UserID       string
	CredentialID []byte
	Label        string
	Data         []byte
	CreatedAt    time.Time
	LastUsedAt   *time.Time
}

// Session is one signed-in browser.
type Session struct {
	ID         string
	UserID     string
	TokenHash  [32]byte
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	MFAAt      *time.Time
	IP         string
	UserAgent  string
	RevokedAt  *time.Time
}

// MFAState says what a fresh session still has to do.
type MFAState string

// What a session needs next.
const (
	// MFADone means a passkey was used in this session.
	MFADone MFAState = "done"
	// MFARequired means the user has passkeys and must use one.
	MFARequired MFAState = "required"
	// MFAEnrolRequired means the user must register their first passkey.
	MFAEnrolRequired MFAState = "enrol_required"
)

// Principal is who a valid session belongs to.
type Principal struct {
	Session  Session
	User     User
	MFA      MFAState
	Passkeys int
}

// Config sets how long staff sessions live. They are shorter than merchants'.
type Config struct {
	Idle       time.Duration
	Absolute   time.Duration
	TouchEvery time.Duration
	// Challenge is how long a passkey ceremony may take.
	Challenge time.Duration
}

// DefaultConfig returns the lifetimes from the design: 15 minutes idle and
// 8 hours in all.
func DefaultConfig() Config {
	return Config{
		Idle: 15 * time.Minute, Absolute: 8 * time.Hour,
		TouchEvery: time.Minute, Challenge: 5 * time.Minute,
	}
}
