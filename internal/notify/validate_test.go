package notify

import (
	"errors"
	"strings"
	"testing"
)

func TestRecipientValidation(t *testing.T) {
	good := map[Channel][]string{
		Email: {"user@example.com", "first.last+tag@sub.example.co.ga"},
		SMS:   {"+24107123456"},
	}
	bad := map[Channel][]string{
		Email: {
			"", "plain", "a@b@c.com", "Name <a@b.com>", "a@b.com, c@d.com", "a@b.com\r\nBcc: x@y.z",
			"a@b.com\nSubject: hi", "a b@c.com", "<a@b.com>", `"a"@b.com`,
			strings.Repeat("a", 250) + "@b.com",
			"a@b.com\x00",
		},
		SMS: {"", "0707", "24107123456", "+24107abc456", "+24107123456\n"},
	}
	for ch, list := range good {
		for _, to := range list {
			if err := validRecipient(ch, to); err != nil {
				t.Errorf("%s %q: %v", ch, to, err)
			}
		}
	}
	for ch, list := range bad {
		for _, to := range list {
			if err := validRecipient(ch, to); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("%s %.30q: got %v", ch, to, err)
			}
		}
	}
}

func TestDataCannotCarryLineBreaksOrControlCharacters(t *testing.T) {
	dirty := []string{"", "line\nbreak", "carriage\rreturn", "nul\x00", "tab\there", "bell\a"}
	for _, v := range dirty {
		if err := cleanData(map[string]string{"k": v}); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%q: got %v", v, err)
		}
	}
	if err := cleanData(map[string]string{"k": strings.Repeat("a", 1025)}); err == nil {
		t.Error("an oversized value must be refused")
	}
	ordinary := map[string]string{
		"business": "Boutique Océane & Fils", "link": "https://x.test/a?b=c",
	}
	if err := cleanData(ordinary); err != nil {
		t.Errorf("ordinary values must pass: %v", err)
	}
}

func TestMaskedRecipientsRevealLittle(t *testing.T) {
	if got := maskRecipient(Email, "jane.doe@example.com"); got != "j***@example.com" {
		t.Errorf("email: %q", got)
	}
	if got := maskRecipient(SMS, "+24107123456"); got != "+241 07 ** ** 56" {
		t.Errorf("sms: %q", got)
	}
}

func TestBackoffDoublesAndCaps(t *testing.T) {
	want := []string{"30s", "1m0s", "2m0s", "4m0s"}
	for i, w := range want {
		if got := Backoff(i + 1).String(); got != w {
			t.Errorf("attempt %d: %s, want %s", i+1, got, w)
		}
	}
	if Backoff(100) != backoffCap || Backoff(0) != backoffBase {
		t.Fatal("backoff must cap at one hour and treat bad input as the first attempt")
	}
}
