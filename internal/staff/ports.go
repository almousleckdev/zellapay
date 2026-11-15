package staff

import (
	"context"
	"time"

	"github.com/zellapay/zellapay/internal/audit"
)

// Effects are audit entries stored in the same transaction as a change.
type Effects struct{ Audit []audit.Pending }

// Repository stores staff, passkeys, sessions and pending ceremonies.
type Repository interface {
	CreateUser(ctx context.Context, u User, fx Effects) error
	FindByEmail(ctx context.Context, email string) (User, error)
	Find(ctx context.Context, id string) (User, error)
	Rehash(ctx context.Context, id, oldHash, newHash string) error

	AddPasskey(ctx context.Context, p Passkey, fx Effects) error
	Passkeys(ctx context.Context, userID string) ([]Passkey, error)
	// UpdatePasskey stores the credential's new counter after a use.
	UpdatePasskey(ctx context.Context, p Passkey, at time.Time) error

	CreateSession(ctx context.Context, s Session) error
	FindSession(ctx context.Context, hash [32]byte) (Session, error)
	Touch(ctx context.Context, id string, at time.Time) error
	SetMFA(ctx context.Context, id string, at time.Time) error
	RevokeSession(ctx context.Context, userID, id string, at time.Time) error

	// SaveChallenge keeps a ceremony's state until it is taken or expires.
	SaveChallenge(ctx context.Context, c Challenge) error
	// TakeChallenge returns and deletes it, so it can be used only once.
	TakeChallenge(
		ctx context.Context, sessionID string, kind ChallengeKind, at time.Time,
	) ([]byte, error)
}

// ChallengeKind is the ceremony a challenge belongs to.
type ChallengeKind string

// Ceremonies.
const (
	ChallengeRegister ChallengeKind = "register"
	ChallengeLogin    ChallengeKind = "login"
)

// Challenge is a ceremony in progress.
type Challenge struct {
	SessionID string
	Kind      ChallengeKind
	Data      []byte
	ExpiresAt time.Time
}
