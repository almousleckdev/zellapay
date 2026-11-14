package notify

import (
	"fmt"
	"net/mail"
	"strings"
	"unicode"

	"github.com/zellapay/zellapay/internal/platform/phone"
)

const (
	maxAddress   = 254
	maxValueSize = 1024
)

// validRecipient checks that to is a plain address for the channel.
func validRecipient(ch Channel, to string) error {
	switch ch {
	case Email:
		return validEmail(to)
	case SMS:
		if !phone.Valid(to) {
			return fmt.Errorf("%w: phone number must be E.164", ErrInvalidInput)
		}
		return nil
	}
	return fmt.Errorf("%w: channel", ErrInvalidInput)
}

// validEmail accepts a bare address and nothing else: never a display name,
// a list or anything with a line break, which are how mail headers get
// injected.
func validEmail(to string) error {
	bad := fmt.Errorf("%w: email address", ErrInvalidInput)
	if to == "" || len(to) > maxAddress || hasControl(to) {
		return bad
	}
	if strings.ContainsAny(to, " ,;<>\"") || strings.Count(to, "@") != 1 {
		return bad
	}
	a, err := mail.ParseAddress(to)
	if err != nil || a.Address != to || a.Name != "" {
		return bad
	}
	return nil
}

// cleanData rejects values that could break a message or its headers.
func cleanData(data map[string]string) error {
	for k, v := range data {
		if v == "" || len(v) > maxValueSize || hasControl(v) {
			return fmt.Errorf("%w: value for %q", ErrInvalidInput, k)
		}
	}
	return nil
}

func hasControl(s string) bool {
	return strings.IndexFunc(s, unicode.IsControl) >= 0
}

// maskRecipient hides most of an address for logs and storage.
func maskRecipient(ch Channel, to string) string {
	if ch == SMS {
		return phone.Mask(to)
	}
	local, domain, _ := strings.Cut(to, "@")
	if local == "" {
		return "***"
	}
	return local[:1] + "***@" + domain
}
