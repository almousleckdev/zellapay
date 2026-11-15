package totp

import (
	"strings"
	"testing"
	"time"
)

// RFC 6238 appendix B, truncated to six digits.
var rfcSecret = []byte("12345678901234567890")

func TestCodeMatchesRFC6238Vectors(t *testing.T) {
	tests := []struct {
		unix int64
		want string
	}{
		{59, "287082"}, {1111111109, "081804"}, {1111111111, "050471"},
		{1234567890, "005924"}, {2000000000, "279037"},
	}
	for _, tc := range tests {
		if got := Code(rfcSecret, Step(time.Unix(tc.unix, 0))); got != tc.want {
			t.Errorf("t=%d: got %s, want %s", tc.unix, got, tc.want)
		}
	}
}

func TestVerifyAcceptsOneStepOfDriftButNotMore(t *testing.T) {
	now := time.Unix(1111111109, 0)
	cur := Step(now)
	for _, tc := range []struct {
		name string
		step int64
		ok   bool
	}{
		{"current", cur, true}, {"previous", cur - 1, true}, {"next", cur + 1, true},
		{"two back", cur - 2, false}, {"two ahead", cur + 2, false},
	} {
		_, ok := Verify(rfcSecret, Code(rfcSecret, tc.step), now, 0)
		if ok != tc.ok {
			t.Errorf("%s: ok = %v, want %v", tc.name, ok, tc.ok)
		}
	}
}

func TestVerifyRefusesAReplayedCode(t *testing.T) {
	now := time.Unix(1111111109, 0)
	code := Code(rfcSecret, Step(now))
	step, ok := Verify(rfcSecret, code, now, 0)
	if !ok {
		t.Fatal("first use refused")
	}
	if _, ok := Verify(rfcSecret, code, now, step); ok {
		t.Fatal("the same code worked twice")
	}
}

func TestVerifyRefusesMalformedCodes(t *testing.T) {
	now := time.Unix(1111111109, 0)
	for _, c := range []string{"", "12345", "1234567", "abcdef", " 050471"} {
		if _, ok := Verify(rfcSecret, c, now, 0); ok {
			t.Errorf("%q accepted", c)
		}
	}
}

func TestURIAndManualKey(t *testing.T) {
	u := URI("ZellaPay", "jane@example.com", rfcSecret)
	for _, part := range []string{
		"otpauth://totp/ZellaPay:jane@example.com",
		"secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "digits=6",
	} {
		if !strings.Contains(u, part) {
			t.Errorf("%q missing %q", u, part)
		}
	}
	if Encode(rfcSecret) != "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" {
		t.Error("manual key wrong")
	}
}
