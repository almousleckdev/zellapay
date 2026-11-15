package identity

import (
	"context"
	"time"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/notify"
)

// Effects are the emails and audit entries that belong to one change. The
// repository stores them in the same transaction, so a change never exists
// without its audit entry, and an email is never sent for a change that was
// rolled back.
type Effects struct {
	Mail  []notify.Prepared
	Audit []audit.Pending
}

// Repository stores users, invitations and email tokens. Each method that
// takes Effects is atomic. Token consumption is a single conditional update,
// so two requests can never use the same link.
type Repository interface {
	// CreateInvitation replaces any open invitation for the same person and
	// merchant, then stores inv.
	CreateInvitation(ctx context.Context, inv Invitation, fx Effects) error
	// FindInvitation returns an open invitation, or ErrInvalidToken.
	FindInvitation(ctx context.Context, hash [32]byte) (Invitation, error)
	// AcceptInvitation uses the invitation and stores u. It returns
	// ErrInvalidToken if the invitation was used meanwhile and ErrEmailTaken
	// if the address already has an account.
	AcceptInvitation(ctx context.Context, a Acceptance, fx Effects) error

	FindUserByEmail(ctx context.Context, email string) (User, error)
	FindUser(ctx context.Context, id string) (User, error)
	// Rehash replaces oldHash with newHash, only if oldHash is still current.
	Rehash(ctx context.Context, id, oldHash, newHash string) error

	// CreateEmailToken replaces any open token of the same purpose.
	CreateEmailToken(ctx context.Context, t EmailToken, fx Effects) error
	// FindEmailToken returns an unused token, or ErrInvalidToken.
	FindEmailToken(ctx context.Context, p Purpose, hash [32]byte) (EmailToken, error)
	// UseEmailToken marks the token used and applies the change it allows.
	UseEmailToken(ctx context.Context, use TokenUse, fx Effects) error
}

// Acceptance is the consumption of an invitation and the account it creates.
type Acceptance struct {
	TokenHash [32]byte
	User      User
	At        time.Time
}

// TokenUse is the consumption of an email token. NewPasswordHash is set only
// for a password reset.
type TokenUse struct {
	Purpose         Purpose
	TokenHash       [32]byte
	UserID          string
	NewPasswordHash string
	At              time.Time
}

// Mailer prepares an email for the outbox.
type Mailer interface {
	Prepare(m notify.Message) (notify.Prepared, error)
}

// Auditor prepares an audit entry.
type Auditor interface {
	Prepare(e audit.Entry) (audit.Pending, error)
}

// Directory answers questions about merchants without exposing their data.
type Directory interface {
	// BusinessName returns the merchant's name, or ErrNotFound.
	BusinessName(ctx context.Context, merchantID string) (string, error)
}
