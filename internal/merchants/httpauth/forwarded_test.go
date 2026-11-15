package httpauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func request(peer string, forwarded ...string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r.RemoteAddr = peer
	for _, f := range forwarded {
		r.Header.Add("X-Forwarded-For", f)
	}
	return r
}

func TestForwarded(t *testing.T) {
	trusted := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("::1/128"),
	}
	type forwardedCase struct {
		name string
		req  *http.Request
		want string
	}
	const proxy, client = "10.0.0.5:1", "198.51.100.7"
	tests := []forwardedCase{
		{"untrusted peer cannot choose", request("203.0.113.9:1", "1.2.3.4"), "203.0.113.9"},
		{"trusted peer without the header", request(proxy), "10.0.0.5"},
		{"trusted proxy reports the client", request(proxy, client), client},
		{"a client cannot prepend a fake", request(proxy, "9.9.9.9, "+client), client},
		{"two proxies in a row", request(proxy, client+", 10.1.1.1"), client},
		{"several headers join in order", request(proxy, "9.9.9.9", client), client},
		{"every hop trusted", request(proxy, "10.2.2.2"), "10.0.0.5"},
		{"garbage falls back to the peer", request(proxy, "not-an-ip"), "10.0.0.5"},
		{"IPv6 loopback proxy", request("[::1]:1", "2001:db8::7"), "2001:db8::7"},
		{"IPv4-mapped peer", request("[::ffff:10.0.0.5]:1", client), client},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Forwarded(trusted)(tc.req).String(); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestForwardedWithNoTrustedProxiesIgnoresTheHeader(t *testing.T) {
	got := Forwarded(nil)(request("10.0.0.5:1", "198.51.100.7"))
	if got.String() != "10.0.0.5" {
		t.Fatalf("got %s", got)
	}
}
