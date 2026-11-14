package phone

import "testing"

func TestValid(t *testing.T) {
	good := []string{"+24107123456", "+33612345678", "+12025550123"}
	bad := []string{
		"", "24107123456", "+0241071234", "+241 07 12 34 56",
		"+2410712", "+241071234567890123", "+24107abc456",
	}
	for _, s := range good {
		if !Valid(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range bad {
		if Valid(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestMask(t *testing.T) {
	cases := map[string]string{
		"+24107123456":  "+241 07 ** ** 56",
		"+241071234567": "+241 07 ** ** * 67",
		"+2410712345":   "+241 07 ** * 45",
		"nonsense":      "***",
		"":              "***",
	}
	for in, want := range cases {
		if got := Mask(in); got != want {
			t.Errorf("Mask(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMaskNeverRevealsTheMiddle(t *testing.T) {
	in := "+24107123456"
	got := Mask(in)
	for _, secret := range []string{"1234", "123", "2345"} {
		if contains(got, secret) {
			t.Fatalf("%q leaks %q", got, secret)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
