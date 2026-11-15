package identity

import (
	"context"
	"fmt"

	"github.com/zellapay/zellapay/internal/audit"
)

// RequestEmailVerification emails a link that confirms the user's address.
// It does nothing for an address that is already verified.
func (s *Service) RequestEmailVerification(ctx context.Context, userID string) error {
	u, err := s.d.Repo.FindUser(ctx, userID)
	if err != nil {
		return err
	}
	if u.EmailVerifiedAt != nil {
		return nil
	}
	return s.issue(ctx, u, verifyKind)
}

// ConfirmEmail marks the address behind a verification link as verified.
func (s *Service) ConfirmEmail(ctx context.Context, token string) error {
	t, err := s.lookup(ctx, verifyKind, token)
	if err != nil {
		return err
	}
	entry, err := s.d.Auditor.Prepare(audit.Entry{
		Actor:  audit.Actor{Kind: audit.ActorMerchantUser, ID: t.UserID},
		Action: verifyKind.action,
		Target: audit.Target{Kind: "merchant_user", ID: t.UserID},
	})
	if err != nil {
		return fmt.Errorf("prepare audit entry: %w", err)
	}
	return s.d.Repo.UseEmailToken(ctx, TokenUse{
		Purpose: PurposeVerifyEmail, TokenHash: t.TokenHash, UserID: t.UserID, At: s.now(),
	}, Effects{Audit: []audit.Pending{entry}})
}
