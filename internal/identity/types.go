// Package identity manages merchant users: who they are, how they prove it
// with a password, and the single-use links that invite them, verify their
// email and reset their password. Sessions and MFA are layered on top.
package identity

import "time"

// Role is what a user may do inside their merchant. The access module turns
// roles into permissions; identity only stores and validates them.
type Role string

// Roles a merchant user can hold.
const (
	RoleOwner     Role = "owner"
	RoleAdmin     Role = "admin"
	RoleDeveloper Role = "developer"
	RoleFinance   Role = "finance"
	RoleSupport   Role = "support"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleDeveloper, RoleFinance, RoleSupport:
		return true
	}
	return false
}

// Status is the state of a user account.
type Status string

// Account states.
const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

// User is a person who works for one merchant.
type User struct {
	ID                string
	MerchantID        string
	Email             string
	Name              string
	Role              Role
	Status            Status
	Locale            string
	PasswordHash      string
	PasswordChangedAt time.Time
	EmailVerifiedAt   *time.Time
	CreatedAt         time.Time
}

// Invitation lets one person create an account for one merchant. Only the
// hash of its token is kept.
type Invitation struct {
	ID         string
	MerchantID string
	Email      string
	Role       Role
	Locale     string
	TokenHash  [32]byte
	InvitedBy  string
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

// Purpose says what an email token may be used for.
type Purpose string

// Email token purposes.
const (
	PurposeVerifyEmail   Purpose = "verify_email"
	PurposeResetPassword Purpose = "reset_password"
)

// EmailToken is a single-use link sent to a user's inbox.
type EmailToken struct {
	ID        string
	UserID    string
	Purpose   Purpose
	TokenHash [32]byte
	ExpiresAt time.Time
	CreatedAt time.Time
}
