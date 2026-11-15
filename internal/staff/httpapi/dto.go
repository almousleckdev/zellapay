package httpapi

import (
	"encoding/json"
	"time"

	"github.com/zellapay/zellapay/internal/staff"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ceremonyRequest carries the browser's answer to a passkey ceremony.
type ceremonyRequest struct {
	Label      string          `json:"label"`
	Credential json.RawMessage `json:"credential"`
}

type loginResponse struct {
	SessionToken string    `json:"session_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	MFA          string    `json:"mfa"`
}

type userJSON struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type sessionJSON struct {
	ID            string     `json:"id"`
	CreatedAt     time.Time  `json:"created_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
	MFAVerifiedAt *time.Time `json:"mfa_verified_at"`
}

type currentResponse struct {
	User     userJSON    `json:"user"`
	Session  sessionJSON `json:"session"`
	MFA      string      `json:"mfa"`
	Passkeys int         `json:"passkeys"`
	// Permissions is what the member's role allows.
	Permissions []string `json:"permissions"`
}

type passkeyJSON struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

type passkeysResponse struct {
	Data []passkeyJSON `json:"data"`
}

func passkeyView(p staff.Passkey) passkeyJSON {
	return passkeyJSON{ID: p.ID, Label: p.Label, CreatedAt: p.CreatedAt, LastUsedAt: p.LastUsedAt}
}
