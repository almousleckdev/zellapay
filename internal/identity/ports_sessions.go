package identity

import (
	"context"
	"time"
)

// SessionRepository stores sessions.
type SessionRepository interface {
	CreateSession(ctx context.Context, s Session) error
	// FindSession returns the session for a token hash, revoked or not, or
	// ErrInvalidToken.
	FindSession(ctx context.Context, hash [32]byte) (Session, error)
	Touch(ctx context.Context, id string, at time.Time) error
	// SetMFA records that the second factor was proven for the session.
	SetMFA(ctx context.Context, id string, at time.Time) error
	// RevokeSession ends one of the user's sessions. It reports false when
	// there was nothing open to end.
	RevokeSession(ctx context.Context, userID, id string, at time.Time) (bool, error)
	// RevokeUserSessions ends all of the user's open sessions except keepID,
	// which may be empty. It returns how many ended.
	RevokeUserSessions(ctx context.Context, userID, keepID string, at time.Time) (int, error)
	ListSessions(ctx context.Context, userID string, at time.Time) ([]Session, error)
}

// MFARepository stores TOTP credentials and recovery codes.
type MFARepository interface {
	// SaveEnrolment stores an unconfirmed secret, replacing an earlier
	// unconfirmed one. It returns ErrMFAAlreadyEnrolled if one is confirmed.
	SaveEnrolment(ctx context.Context, c MFACredential) error
	FindMFA(ctx context.Context, userID string) (MFACredential, error)
	// Confirm activates the credential, stores the recovery codes and
	// queues the effects, atomically.
	Confirm(ctx context.Context, c Confirmation, fx Effects) error
	// AdvanceStep stores the newest accepted time step. It reports false if
	// step is not newer than the stored one, which blocks replays.
	AdvanceStep(ctx context.Context, userID string, step int64) (bool, error)
	// UseRecoveryCode spends one code. It reports false if it is unknown or
	// already spent, and returns how many codes remain.
	UseRecoveryCode(ctx context.Context, u RecoveryUse, fx Effects) (ok bool, left int, err error)
}

// MFACredential is a user's TOTP secret. SecretEnc is sealed.
type MFACredential struct {
	UserID      string
	SecretEnc   string
	ConfirmedAt *time.Time
	LastStep    int64
	CreatedAt   time.Time
}

// Confirmation activates a credential and replaces the recovery codes.
type Confirmation struct {
	UserID     string
	Step       int64
	CodeHashes [][32]byte
	At         time.Time
}

// RecoveryUse spends one recovery code.
type RecoveryUse struct {
	UserID string
	Hash   [32]byte
	At     time.Time
}

// SecretBox seals and opens secrets at rest.
type SecretBox interface {
	Seal(plaintext, aad []byte) (string, error)
	Open(token string, aad []byte) ([]byte, error)
}
