package identity

import (
	"context"
	"strings"
)

// PasswordChangeInput is a request to change the signed-in user's password.
type PasswordChangeInput struct {
	Token   string
	Current string
	New     string
	IP      string
}

// ChangePassword checks the current password, throttled like a sign-in, then
// sets the new one and ends the user's other sessions. The caller must have
// passed the access policy's step-up check first.
func (s *Sessions) ChangePassword(ctx context.Context, in PasswordChangeInput) error {
	p, err := s.Resolve(ctx, in.Token)
	if err != nil {
		return err
	}
	a := Attempt{Realm: RealmPassword, Account: strings.ToLower(p.User.Email), IP: in.IP}
	if err := s.guard(ctx, a); err != nil {
		return err
	}
	if _, err := s.d.Accounts.Authenticate(ctx, p.User.Email, in.Current); err != nil {
		return s.failed(ctx, a, err)
	}
	s.succeeded(ctx, a)
	return s.d.Accounts.ChangePassword(ctx, ChangePasswordInput{
		UserID: p.User.ID, NewPassword: in.New, KeepSessionID: p.Session.ID,
	})
}

// RegenerateRecoveryCodes issues a new set of recovery codes for the user
// behind the session. They are returned once.
func (s *Sessions) RegenerateRecoveryCodes(ctx context.Context, token string) ([]string, error) {
	p, err := s.Resolve(ctx, token)
	if err != nil {
		return nil, err
	}
	return s.d.Factor.RegenerateRecoveryCodes(ctx, p.User.ID)
}

// RemoveAuthenticator deletes the user's authenticator and ends their other
// sessions. This session stays signed in but must enrol a new one.
func (s *Sessions) RemoveAuthenticator(ctx context.Context, token string) error {
	p, err := s.Resolve(ctx, token)
	if err != nil {
		return err
	}
	return s.d.Factor.Remove(ctx, p.User.ID, p.Session.ID)
}
