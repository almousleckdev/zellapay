package staff

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/zellapay/zellapay/internal/identity"
)

// BeginLogin starts proving a passkey for a session that has not used one.
func (s *Service) BeginLogin(ctx context.Context, token string) (json.RawMessage, error) {
	p, err := s.Resolve(ctx, token)
	if err != nil {
		return nil, err
	}
	if p.MFA != MFARequired {
		return nil, fmt.Errorf("%w: no passkey is pending", identity.ErrInvalidInput)
	}
	user, err := s.ceremonyUser(ctx, p.User)
	if err != nil {
		return nil, err
	}
	assertion, session, err := s.d.WebAuthn.BeginLogin(user)
	if err != nil {
		return nil, fmt.Errorf("begin login: %w", err)
	}
	if err := s.keep(ctx, p.Session.ID, ChallengeLogin, session); err != nil {
		return nil, err
	}
	return json.Marshal(assertion)
}

// FinishLogin checks the browser's signed answer. A passkey that looks cloned
// is refused. Wrong answers are counted per user.
func (s *Service) FinishLogin(ctx context.Context, in FinishInput) error {
	p, err := s.Resolve(ctx, in.Token)
	if err != nil {
		return err
	}
	a := attempt(identity.RealmMFA, p.User.ID, in.IP)
	if err := s.guard(ctx, a); err != nil {
		return err
	}
	if err := s.verifyLogin(ctx, p, in.Body); err != nil {
		return s.failed(ctx, a, err)
	}
	s.succeeded(ctx, a)
	return s.d.Repo.SetMFA(ctx, p.Session.ID, s.d.Clock.Now())
}

func (s *Service) verifyLogin(ctx context.Context, p Principal, body []byte) error {
	session, err := s.take(ctx, p.Session.ID, ChallengeLogin)
	if err != nil {
		return err
	}
	keys, err := s.d.Repo.Passkeys(ctx, p.User.ID)
	if err != nil {
		return err
	}
	user, err := newCeremonyUser(p.User, keys)
	if err != nil {
		return err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(body)
	if err != nil {
		return identity.ErrInvalidCode
	}
	cred, err := s.d.WebAuthn.ValidateLogin(user, session, parsed)
	if err != nil || cred.Authenticator.CloneWarning {
		return identity.ErrInvalidCode
	}
	return s.recordUse(ctx, keys, cred)
}

// recordUse stores the passkey's new signature counter, which is how a cloned
// authenticator is noticed later.
func (s *Service) recordUse(ctx context.Context, keys []Passkey, cred *webauthn.Credential) error {
	data, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	for _, k := range keys {
		if string(k.CredentialID) == string(cred.ID) {
			k.Data = data
			return s.d.Repo.UpdatePasskey(ctx, k, s.d.Clock.Now())
		}
	}
	return identity.ErrInvalidCode
}
