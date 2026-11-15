package keysapi

import (
	"time"

	"github.com/zellapay/zellapay/internal/merchants"
)

type createRequest struct {
	Label  string   `json:"label"`
	Scopes []string `json:"scopes"`
}

type rotateRequest struct {
	// OverlapHours is how long the old key keeps working. Default 24.
	OverlapHours int `json:"overlap_hours"`
}

type keyJSON struct {
	ID        string     `json:"id"`
	Label     string     `json:"label"`
	Prefix    string     `json:"prefix"`
	Mode      string     `json:"mode"`
	Scopes    []string   `json:"scopes"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`
}

func keyView(k merchants.APIKey) keyJSON {
	scopes := make([]string, len(k.Scopes))
	for i, s := range k.Scopes {
		scopes[i] = string(s)
	}
	return keyJSON{
		ID: k.ID, Label: k.Label, Prefix: k.LookupID, Mode: string(k.Mode), Scopes: scopes,
		CreatedAt: k.CreatedAt, ExpiresAt: k.ExpiresAt, RevokedAt: k.RevokedAt,
	}
}

type listResponse struct {
	Data []keyJSON `json:"data"`
}

// issuedResponse carries the secret. It is the only time it is ever shown.
type issuedResponse struct {
	Key    keyJSON `json:"key"`
	Secret string  `json:"secret"`
}
