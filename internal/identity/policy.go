package identity

import (
	_ "embed"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	minPasswordLength = 12
	maxPasswordLength = 128
)

//go:embed common_passwords.txt
var commonRaw string

var common = loadCommon()

func loadCommon() map[string]struct{} {
	set := map[string]struct{}{}
	for _, line := range strings.Split(commonRaw, "\n") {
		if w := strings.TrimSpace(line); w != "" {
			set[w] = struct{}{}
		}
	}
	return set
}

// CheckPassword applies the password policy. There are no composition rules:
// length and not being a well-known password matter, not symbols. userHints
// are the person's own words (email, name) that must not be the password.
func CheckPassword(password string, userHints ...string) error {
	n := norm.NFKC.String(password)
	length := utf8.RuneCountInString(n)
	if length < minPasswordLength {
		return weak("too short")
	}
	if length > maxPasswordLength {
		return weak("too long")
	}
	lower := strings.ToLower(n)
	if _, bad := common[lower]; bad || repeatsOneRune(n) {
		return weak("too common")
	}
	for _, hint := range userHints {
		if h := strings.ToLower(strings.TrimSpace(hint)); len(h) >= 4 && lower == h {
			return weak("matches your details")
		}
	}
	return nil
}

func repeatsOneRune(s string) bool {
	first, _ := utf8.DecodeRuneInString(s)
	return strings.Count(s, string(first)) == utf8.RuneCountInString(s)
}
