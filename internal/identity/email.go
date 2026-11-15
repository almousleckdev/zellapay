package identity

import (
	"net/mail"
	"strings"
)

const maxEmailLength = 254

// NormalizeEmail returns the canonical form of an address: trimmed and
// lower case. It accepts a bare address only, never a display name or a list.
func NormalizeEmail(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" || len(s) > maxEmailLength {
		return "", invalid("email")
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s || addr.Name != "" {
		return "", invalid("email")
	}
	return s, nil
}
