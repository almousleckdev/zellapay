package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/platform/ids"
)

const maxNameLength = 100

// AcceptInput is what a person submits on the invitation page.
type AcceptInput struct {
	Token    string
	Name     string
	Password string
}

// AcceptInvitation creates the account an invitation allows. Following the
// link proves the person controls the inbox, so the email counts as verified.
func (s *Service) AcceptInvitation(ctx context.Context, in AcceptInput) (User, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > maxNameLength {
		return User{}, invalid("name")
	}
	hash := hashToken(in.Token)
	inv, err := s.d.Repo.FindInvitation(ctx, hash)
	if err != nil {
		return User{}, err
	}
	now := s.now()
	if !inv.ExpiresAt.After(now) {
		return User{}, ErrInvalidToken
	}
	u, err := s.newUser(inv, name, in.Password)
	if err != nil {
		return User{}, err
	}
	entry, err := s.d.Auditor.Prepare(audit.Entry{
		Actor:  audit.Actor{Kind: audit.ActorMerchantUser, ID: u.ID},
		Action: "user.created", Target: audit.Target{Kind: "merchant_user", ID: u.ID},
		Change: map[string]string{"merchant": u.MerchantID, "role": string(u.Role)},
	})
	if err != nil {
		return User{}, fmt.Errorf("prepare audit entry: %w", err)
	}
	err = s.d.Repo.AcceptInvitation(ctx,
		Acceptance{TokenHash: hash, User: u, At: now},
		Effects{Audit: []audit.Pending{entry}})
	if errors.Is(err, ErrInvalidToken) || errors.Is(err, ErrEmailTaken) {
		return User{}, err
	}
	if err != nil {
		return User{}, fmt.Errorf("accept invitation: %w", err)
	}
	return u, nil
}

// newUser checks the password and builds the account an invitation allows.
func (s *Service) newUser(inv Invitation, name, password string) (User, error) {
	if err := CheckPassword(password, inv.Email, name); err != nil {
		return User{}, err
	}
	passwordHash, err := s.d.Hasher.Hash(password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}
	now := s.now()
	return User{
		ID: ids.New("usr_"), MerchantID: inv.MerchantID, Email: inv.Email,
		Name: name, Role: inv.Role, Status: StatusActive, Locale: inv.Locale,
		PasswordHash: passwordHash, PasswordChangedAt: now,
		EmailVerifiedAt: &now, CreatedAt: now,
	}, nil
}
