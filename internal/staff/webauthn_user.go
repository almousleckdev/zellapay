package staff

import (
	"encoding/json"
	"fmt"

	"github.com/go-webauthn/webauthn/webauthn"
)

// ceremonyUser lets the WebAuthn library see a staff member and their keys.
type ceremonyUser struct {
	user  User
	creds []webauthn.Credential
}

var _ webauthn.User = (*ceremonyUser)(nil)

func newCeremonyUser(u User, keys []Passkey) (*ceremonyUser, error) {
	creds := make([]webauthn.Credential, 0, len(keys))
	for _, k := range keys {
		var c webauthn.Credential
		if err := json.Unmarshal(k.Data, &c); err != nil {
			return nil, fmt.Errorf("read passkey %s: %w", k.ID, err)
		}
		creds = append(creds, c)
	}
	return &ceremonyUser{user: u, creds: creds}, nil
}

func (c *ceremonyUser) WebAuthnID() []byte                         { return []byte(c.user.ID) }
func (c *ceremonyUser) WebAuthnName() string                       { return c.user.Email }
func (c *ceremonyUser) WebAuthnDisplayName() string                { return c.user.Name }
func (c *ceremonyUser) WebAuthnCredentials() []webauthn.Credential { return c.creds }
