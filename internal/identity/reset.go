package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/zellapay/zellapay/internal/audit"
)

// RequestPasswordReset emails a reset link when the address has an active
// account. It returns nil either way, so the caller can show one message and
// nobody can use this to find out who has an account. A malformed address is
// reported, since that says nothing about any account.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	addr, err := NormalizeEmail(email)
	if err != nil {
		return err
	}
	u, err := s.d.Repo.FindUserByEmail(ctx, addr)
	if errors.Is(err, ErrNotFound) || (err == nil && u.Status != StatusActive) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.issue(ctx, u, resetKind)
}

// ResetInput is what a person submits on the reset page.
type ResetInput struct {
	Token       string
	NewPassword string
}

// CompletePasswordReset sets a new password. Following the link also proves
// the inbox, so it verifies the email. It does not bypass MFA, which a later
// sign-in still requires.
func (s *Service) CompletePasswordReset(ctx context.Context, in ResetInput) error {
	t, err := s.lookup(ctx, resetKind, in.Token)
	if err != nil {
		return err
	}
	u, err := s.d.Repo.FindUser(ctx, t.UserID)
	if err != nil {
		return err
	}
	if err := CheckPassword(in.NewPassword, u.Email, u.Name); err != nil {
		return err
	}
	passwordHash, err := s.d.Hasher.Hash(in.NewPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	entry, err := s.d.Auditor.Prepare(audit.Entry{
		Actor:  audit.Actor{Kind: audit.ActorMerchantUser, ID: u.ID},
		Action: "password.reset", Target: audit.Target{Kind: "merchant_user", ID: u.ID},
	})
	if err != nil {
		return fmt.Errorf("prepare audit entry: %w", err)
	}
	return s.d.Repo.UseEmailToken(ctx, TokenUse{
		Purpose: PurposeResetPassword, TokenHash: t.TokenHash, UserID: u.ID,
		NewPasswordHash: passwordHash, At: s.now(),
	}, Effects{Audit: []audit.Pending{entry}})
}
