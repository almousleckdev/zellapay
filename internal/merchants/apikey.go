package merchants

import (
	"net/netip"
	"time"

	"github.com/zellapay/zellapay/internal/audit"
)

// APIKey is the stored form of a key. The secret itself is never kept.
type APIKey struct {
	ID          string
	MerchantID  string
	LookupID    string
	SecretHash  []byte
	Mode        Mode
	Scopes      []Scope
	IPAllowlist []netip.Prefix
	CreatedAt   time.Time
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
	// Label is the name the merchant gave the key. It is not a secret.
	Label string
}

// IssuedKey is returned once, when a key is created. Secret is the only
// time the full key is available.
type IssuedKey struct {
	Key    APIKey
	Secret string
}

// KeySpec describes a key to issue.
type KeySpec struct {
	MerchantID  string
	Mode        Mode
	Scopes      []Scope
	IPAllowlist []netip.Prefix
	ExpiresAt   *time.Time
	Label       string
	// By is who is creating the key. When set, the creation is audited in
	// the same transaction, which needs the service to have an auditor.
	By *audit.Actor
}

// Principal is the authenticated caller of a request.
type Principal struct {
	MerchantID string
	KeyID      string
	Mode       Mode
	Scopes     []Scope
}

// Has reports whether the principal holds scope s.
func (p Principal) Has(s Scope) bool {
	for _, have := range p.Scopes {
		if have == s {
			return true
		}
	}
	return false
}

func (k APIKey) usableAt(now time.Time) bool {
	if k.RevokedAt != nil {
		return false
	}
	return k.ExpiresAt == nil || now.Before(*k.ExpiresAt)
}

func (k APIKey) allowsIP(ip netip.Addr) bool {
	if len(k.IPAllowlist) == 0 {
		return true
	}
	for _, p := range k.IPAllowlist {
		if p.Contains(ip.Unmap()) {
			return true
		}
	}
	return false
}
