package identity

import (
	"errors"
	"fmt"
)

// Errors callers handle. None reveals whether an account exists.
var (
	// ErrInvalidInput means a field is missing, malformed or out of range.
	ErrInvalidInput = errors.New("identity: invalid input")
	// ErrWeakPassword means the password breaks the policy.
	ErrWeakPassword = errors.New("identity: password does not meet the policy")
	// ErrInvalidToken covers every way a link can be unusable: unknown,
	// expired, already used or replaced. They look the same on purpose.
	ErrInvalidToken = errors.New("identity: link is invalid or has expired")
	// ErrInvalidCredentials is the only sign-in failure callers see.
	ErrInvalidCredentials = errors.New("identity: invalid email or password")
	// ErrEmailTaken means an account already exists for the email.
	ErrEmailTaken = errors.New("identity: an account already exists")
	// ErrNotFound means no such user or merchant.
	ErrNotFound = errors.New("identity: not found")
)

func invalid(field string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, field)
}

func weak(reason string) error {
	return fmt.Errorf("%w: %s", ErrWeakPassword, reason)
}

// Errors specific to sessions and the second factor.
var (
	// ErrInvalidSession covers every way a session cannot be used: unknown,
	// expired, idle too long or ended. They look the same on purpose.
	ErrInvalidSession = errors.New("identity: session is invalid or has expired")
	// ErrInvalidCode means a one-time or recovery code was wrong or reused.
	ErrInvalidCode = errors.New("identity: invalid code")
	// ErrMFAAlreadyEnrolled means a second factor is already active.
	ErrMFAAlreadyEnrolled = errors.New("identity: MFA is already enrolled")
	// ErrNoEnrolment means no enrolment is waiting to be confirmed.
	ErrNoEnrolment = errors.New("identity: no MFA enrolment in progress")
)
