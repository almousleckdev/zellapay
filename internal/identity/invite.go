package identity

import (
	"context"
	"fmt"
	"strconv"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/notify"
	"github.com/zellapay/zellapay/internal/platform/ids"
)

// InviteInput describes who to invite and who is inviting them.
type InviteInput struct {
	MerchantID string
	Email      string
	Role       Role
	Locale     string
	InvitedBy  audit.Actor
}

// Invite emails a single-use link that lets one person create an account for
// a merchant. A newer invitation to the same person replaces the older one.
// Who may invite whom is decided by the caller, before this is called.
func (s *Service) Invite(ctx context.Context, in InviteInput) (Invitation, error) {
	email, locale, err := checkInvite(in)
	if err != nil {
		return Invitation{}, err
	}
	business, err := s.d.Directory.BusinessName(ctx, in.MerchantID)
	if err != nil {
		return Invitation{}, err
	}
	token, hash, err := newToken()
	if err != nil {
		return Invitation{}, err
	}
	now := s.now()
	inv := Invitation{
		ID: ids.New("inv_"), MerchantID: in.MerchantID, Email: email,
		Role: in.Role, Locale: locale, TokenHash: hash,
		InvitedBy: in.InvitedBy.ID, ExpiresAt: now.Add(s.cfg.InvitationTTL), CreatedAt: now,
	}
	fx, err := s.inviteEffects(inv, business, token, in.InvitedBy)
	if err != nil {
		return Invitation{}, err
	}
	if err := s.d.Repo.CreateInvitation(ctx, inv, fx); err != nil {
		return Invitation{}, fmt.Errorf("create invitation: %w", err)
	}
	return inv, nil
}

func checkInvite(in InviteInput) (email, locale string, err error) {
	if in.MerchantID == "" || !in.Role.Valid() {
		return "", "", invalid("merchant or role")
	}
	if email, err = NormalizeEmail(in.Email); err != nil {
		return "", "", err
	}
	if locale, err = cleanLocale(in.Locale); err != nil {
		return "", "", err
	}
	return email, locale, nil
}

func cleanLocale(l string) (string, error) {
	switch l {
	case "":
		return notify.LocaleFrench, nil
	case notify.LocaleFrench, notify.LocaleEnglish:
		return l, nil
	}
	return "", invalid("locale")
}

func (s *Service) inviteEffects(
	inv Invitation, business, token string, by audit.Actor,
) (Effects, error) {
	mail, err := s.d.Mailer.Prepare(notify.Message{
		To: inv.Email, Template: "invitation", Locale: inv.Locale,
		Data: map[string]string{
			"business": business,
			"days":     strconv.Itoa(int(s.cfg.InvitationTTL.Hours() / 24)),
			"link":     s.link("/accept-invitation", token),
		},
		DedupeKey: "invitation:" + inv.ID,
	})
	if err != nil {
		return Effects{}, fmt.Errorf("prepare invitation email: %w", err)
	}
	entry, err := s.d.Auditor.Prepare(audit.Entry{
		Actor: by, Action: "invitation.created",
		Target: audit.Target{Kind: "invitation", ID: inv.ID},
		Change: map[string]string{"merchant": inv.MerchantID, "role": string(inv.Role)},
	})
	if err != nil {
		return Effects{}, fmt.Errorf("prepare audit entry: %w", err)
	}
	return Effects{Mail: []notify.Prepared{mail}, Audit: []audit.Pending{entry}}, nil
}
