package identity

import "testing"

func TestNormalizeEmail(t *testing.T) {
	good := []struct{ in, want string }{
		{"Jane@Example.COM ", "jane@example.com"},
		{"a.b+tag@mail.ga", "a.b+tag@mail.ga"},
	}
	for _, tc := range good {
		if got, err := NormalizeEmail(tc.in); err != nil || got != tc.want {
			t.Errorf("%q: got %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	for _, in := range []string{
		"", "plain", "a@b@c", "Jane <j@x.com>", "j@x.com, k@x.com", "<j@x.com>",
	} {
		if _, err := NormalizeEmail(in); err == nil {
			t.Errorf("%q accepted", in)
		}
	}
}
