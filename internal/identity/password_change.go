package identity

import (
	"context"
	"fmt"

	"github.com/zellapay/zellapay/internal/audit"
)

// ChangePasswordInput names the user and the new password. Checking the
// current password is the caller's job: it must be throttled like a sign-in.
type ChangePasswordInput struct {
	UserID      string
	NewPassword string
	// KeepSessionID stays signed in; the user's other sessions end.
	KeepSessionID string
}

// ChangePassword applies the password policy, stores the new hash and ends
// every other session, so a stolen session cannot outlive the change.
func (s *Service) ChangePassword(ctx context.Context, in ChangePasswordInput) error {
	u, err := s.d.Repo.FindUser(ctx, in.UserID)
	if err != nil {
		return err
	}
	if err := CheckPassword(in.NewPassword, u.Email, u.Name); err != nil {
		return err
	}
	hash, err := s.d.Hasher.Hash(in.NewPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	entry, err := s.d.Auditor.Prepare(audit.Entry{
		Actor:  audit.Actor{Kind: audit.ActorMerchantUser, ID: u.ID},
		Action: "password.changed", Target: audit.Target{Kind: "merchant_user", ID: u.ID},
	})
	if err != nil {
		return fmt.Errorf("prepare audit entry: %w", err)
	}
	return s.d.Repo.ChangePassword(ctx, PasswordChange{
		UserID: u.ID, PasswordHash: hash, KeepSessionID: in.KeepSessionID, At: s.now(),
	}, Effects{Audit: []audit.Pending{entry}})
}
