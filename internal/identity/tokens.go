package identity

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/notify"
	"github.com/zellapay/zellapay/internal/platform/ids"
)

// tokenKind is what differs between the two email-link flows.
type tokenKind struct {
	purpose  Purpose
	template string
	path     string
	action   string
	ttlKey   string // the placeholder the template uses for the lifetime
}

var (
	verifyKind = tokenKind{PurposeVerifyEmail, "email_verification",
		"/verify-email", "email.verified", "hours"}
	resetKind = tokenKind{PurposeResetPassword, "password_reset",
		"/reset-password", "password.reset_requested", "minutes"}
)

func (s *Service) lifetime(k tokenKind) (ttl time.Duration, shown string) {
	if k.purpose == PurposeVerifyEmail {
		return s.cfg.VerificationTTL, strconv.Itoa(int(s.cfg.VerificationTTL.Hours()))
	}
	return s.cfg.ResetTTL, strconv.Itoa(int(s.cfg.ResetTTL.Minutes()))
}

// issue stores a new link for u and queues its email. Any open link of the
// same kind stops working.
func (s *Service) issue(ctx context.Context, u User, k tokenKind) error {
	token, hash, err := newToken()
	if err != nil {
		return err
	}
	ttl, shown := s.lifetime(k)
	now := s.now()
	t := EmailToken{
		ID: ids.New("etk_"), UserID: u.ID, Purpose: k.purpose, TokenHash: hash,
		ExpiresAt: now.Add(ttl), CreatedAt: now,
	}
	mail, err := s.d.Mailer.Prepare(notify.Message{
		To: u.Email, Template: k.template, Locale: u.Locale,
		Data:      map[string]string{"link": s.link(k.path, token), k.ttlKey: shown},
		DedupeKey: "email-token:" + t.ID,
	})
	if err != nil {
		return fmt.Errorf("prepare email: %w", err)
	}
	fx := Effects{Mail: []notify.Prepared{mail}}
	if k.purpose == PurposeResetPassword {
		if fx.Audit, err = s.systemEntry(k.action, u); err != nil {
			return err
		}
	}
	return s.d.Repo.CreateEmailToken(ctx, t, fx)
}

func (s *Service) systemEntry(action string, u User) ([]audit.Pending, error) {
	e, err := s.d.Auditor.Prepare(audit.Entry{
		Actor:  audit.Actor{Kind: audit.ActorSystem, ID: "identity"},
		Action: action, Target: audit.Target{Kind: "merchant_user", ID: u.ID},
	})
	if err != nil {
		return nil, fmt.Errorf("prepare audit entry: %w", err)
	}
	return []audit.Pending{e}, nil
}

// lookup finds the open token and checks it has not expired.
func (s *Service) lookup(ctx context.Context, k tokenKind, token string) (EmailToken, error) {
	t, err := s.d.Repo.FindEmailToken(ctx, k.purpose, hashToken(token))
	if err != nil {
		return EmailToken{}, err
	}
	if !t.ExpiresAt.After(s.now()) {
		return EmailToken{}, ErrInvalidToken
	}
	return t, nil
}
