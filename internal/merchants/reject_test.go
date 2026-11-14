package merchants

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"
)

// scenario prepares a situation and returns the raw key to present.
type scenario func(t *testing.T, e env) string

func wrongSecret(t *testing.T, e env) string {
	return corrupt(e.issue(t, nil).Secret)
}

func unknownKey(t *testing.T, _ env) string {
	other := newEnv(t, ModeTest)
	return other.issue(t, nil).Secret
}

func revokedKey(t *testing.T, e env) string {
	k := e.issue(t, nil)
	if err := e.svc.RevokeKey(context.Background(), e.mer.ID, k.Key.ID); err != nil {
		t.Fatal(err)
	}
	return k.Secret
}

func expiredKey(t *testing.T, e env) string {
	exp := testNow.Add(time.Hour)
	k := e.issue(t, func(s *KeySpec) { s.ExpiresAt = &exp })
	e.clock.now = exp
	return k.Secret
}

func wrongModeKey(t *testing.T, e env) string {
	return e.issue(t, func(s *KeySpec) { s.Mode = ModeLive }).Secret
}

func suspendedMerchant(t *testing.T, e env) string {
	k := e.issue(t, nil)
	if err := e.svc.SetStatus(context.Background(), e.mer.ID, StatusSuspended); err != nil {
		t.Fatal(err)
	}
	return k.Secret
}

func disallowedIP(t *testing.T, e env) string {
	return e.issue(t, func(s *KeySpec) {
		s.IPAllowlist = []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")}
	}).Secret
}

func malformed(*testing.T, env) string { return "not-a-key" }

func TestAuthenticateRejects(t *testing.T) {
	cases := []struct {
		name string
		want error
		run  scenario
	}{
		{"wrong secret", ErrInvalidKey, wrongSecret},
		{"unknown key", ErrInvalidKey, unknownKey},
		{"revoked", ErrInvalidKey, revokedKey},
		{"expired", ErrInvalidKey, expiredKey},
		{"wrong mode", ErrWrongMode, wrongModeKey},
		{"suspended merchant", ErrMerchantInactive, suspendedMerchant},
		{"ip not allowed", ErrIPNotAllowed, disallowedIP},
		{"malformed", ErrInvalidKey, malformed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, ModeTest)
			if _, err := e.auth(tc.run(t, e)); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

// corrupt changes the final character so the result always differs.
func corrupt(secret string) string {
	swap := byte('A')
	if secret[len(secret)-1] == swap {
		swap = 'B'
	}
	return secret[:len(secret)-1] + string(swap)
}
