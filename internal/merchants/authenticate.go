package merchants

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/netip"
)

// zeroHash is compared against when a lookup misses, so a missing key and
// a wrong secret take comparable time.
var zeroHash = make([]byte, 32)

// Authenticate verifies a raw API key presented by ip and returns the
// caller. All failures before a key is positively identified return
// ErrInvalidKey so callers cannot probe which keys exist.
func (s *Service) Authenticate(
	ctx context.Context, raw string, ip netip.Addr,
) (Principal, error) {
	parts, err := parseKey(raw)
	if err != nil {
		return Principal{}, err
	}
	stored, err := s.repo.KeyByLookup(ctx, parts.lookup)
	if errors.Is(err, ErrNotFound) {
		_ = subtle.ConstantTimeCompare(parts.hash(), zeroHash)
		return Principal{}, ErrInvalidKey
	}
	if err != nil {
		return Principal{}, err
	}
	if subtle.ConstantTimeCompare(parts.hash(), stored.Key.SecretHash) != 1 {
		return Principal{}, ErrInvalidKey
	}
	return s.authorize(stored, ip)
}

// authorize applies the policy checks to a key whose secret matched.
func (s *Service) authorize(st StoredKey, ip netip.Addr) (Principal, error) {
	k := st.Key
	switch {
	case !k.usableAt(s.clock.Now()):
		return Principal{}, ErrInvalidKey
	case k.Mode != s.mode:
		return Principal{}, ErrWrongMode
	case st.MerchantStatus != StatusActive:
		return Principal{}, ErrMerchantInactive
	case !k.allowsIP(ip):
		return Principal{}, ErrIPNotAllowed
	}
	return Principal{
		MerchantID: k.MerchantID,
		KeyID:      k.ID,
		Mode:       k.Mode,
		Scopes:     k.Scopes,
	}, nil
}
