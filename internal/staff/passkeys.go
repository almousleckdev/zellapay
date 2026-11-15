package staff

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/ids"
)

// RegistrationOptions is what the browser needs to create a passkey, as the
// JSON the WebAuthn API expects.
type RegistrationOptions = json.RawMessage

// BeginRegistration starts adding a passkey. A session that has used no
// passkey may add its first; after that the session must have used one.
func (s *Service) BeginRegistration(
	ctx context.Context, token string,
) (RegistrationOptions, error) {
	p, err := s.Resolve(ctx, token)
	if err != nil {
		return nil, err
	}
	if p.MFA == MFARequired {
		return nil, fmt.Errorf("%w: use a passkey first", identity.ErrInvalidInput)
	}
	user, err := s.ceremonyUser(ctx, p.User)
	if err != nil {
		return nil, err
	}
	creation, session, err := s.d.WebAuthn.BeginRegistration(user,
		webauthn.WithExclusions(webauthn.Credentials(user.creds).CredentialDescriptors()),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementDiscouraged),
	)
	if err != nil {
		return nil, fmt.Errorf("begin registration: %w", err)
	}
	if err := s.keep(ctx, p.Session.ID, ChallengeRegister, session); err != nil {
		return nil, err
	}
	return json.Marshal(creation)
}

// FinishInput is the browser's answer to a ceremony.
type FinishInput struct {
	Token string
	// Body is the JSON the browser's WebAuthn call produced.
	Body []byte
	// Label names a new passkey, for example "YubiKey". Registration only.
	Label string
	IP    string
}

// FinishRegistration stores the passkey the browser created. Registering the
// first one counts as using a passkey, since it proves the person holds it.
func (s *Service) FinishRegistration(ctx context.Context, in FinishInput) (Passkey, error) {
	p, err := s.Resolve(ctx, in.Token)
	if err != nil {
		return Passkey{}, err
	}
	session, err := s.take(ctx, p.Session.ID, ChallengeRegister)
	if err != nil {
		return Passkey{}, err
	}
	user, err := s.ceremonyUser(ctx, p.User)
	if err != nil {
		return Passkey{}, err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(in.Body)
	if err != nil {
		return Passkey{}, identity.ErrInvalidCode
	}
	cred, err := s.d.WebAuthn.CreateCredential(user, session, parsed)
	if err != nil {
		return Passkey{}, identity.ErrInvalidCode
	}
	key, err := s.savePasskey(ctx, p, cred, in.Label)
	if err != nil {
		return Passkey{}, err
	}
	if p.MFA == MFAEnrolRequired {
		err = s.d.Repo.SetMFA(ctx, p.Session.ID, s.d.Clock.Now())
	}
	return key, err
}

func (s *Service) savePasskey(
	ctx context.Context, p Principal, cred *webauthn.Credential, label string,
) (Passkey, error) {
	data, err := json.Marshal(cred)
	if err != nil {
		return Passkey{}, err
	}
	if label == "" || len(label) > 64 {
		label = "Passkey"
	}
	key := Passkey{
		ID: ids.New("pky_"), UserID: p.User.ID, CredentialID: cred.ID, Label: label,
		Data: data, CreatedAt: s.d.Clock.Now(),
	}
	entry, err := s.d.Auditor.Prepare(audit.Entry{
		Actor:  audit.Actor{Kind: audit.ActorStaff, ID: p.User.ID},
		Action: "staff.passkey_registered", Reason: "registered own passkey",
		Target: audit.Target{Kind: "staff_user", ID: p.User.ID},
	})
	if err != nil {
		return Passkey{}, fmt.Errorf("prepare audit entry: %w", err)
	}
	if err := s.d.Repo.AddPasskey(ctx, key, Effects{Audit: []audit.Pending{entry}}); err != nil {
		return Passkey{}, err
	}
	return key, nil
}

func (s *Service) ceremonyUser(ctx context.Context, u User) (*ceremonyUser, error) {
	keys, err := s.d.Repo.Passkeys(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	return newCeremonyUser(u, keys)
}

// ListPasskeys returns the passkeys of the member behind the session.
func (s *Service) ListPasskeys(ctx context.Context, token string) ([]Passkey, error) {
	p, err := s.Resolve(ctx, token)
	if err != nil {
		return nil, err
	}
	return s.d.Repo.Passkeys(ctx, p.User.ID)
}
