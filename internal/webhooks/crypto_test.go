package webhooks

import (
	"errors"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

func TestSignatureVerifies(t *testing.T) {
	body := []byte(`{"id":"evt_1"}`)
	h := Sign("whsec_abc", t0, body)
	if err := NewVerifier("whsec_abc").Verify(h, body, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "t="+strconv.FormatInt(t0.Unix(), 10)+",v1=") {
		t.Fatalf("unexpected format %q", h)
	}
}

func TestSignatureRejectsTampering(t *testing.T) {
	body := []byte(`{"amount":100}`)
	h := Sign("whsec_abc", t0, body)
	cases := map[string]struct {
		secret, header string
		body           []byte
	}{
		"changed body":   {"whsec_abc", h, []byte(`{"amount":999}`)},
		"wrong secret":   {"whsec_other", h, body},
		"changed time":   {"whsec_abc", shiftTime(h), body},
		"changed digest": {"whsec_abc", h[:len(h)-2] + "00", body},
	}
	for name, c := range cases {
		err := NewVerifier(c.secret).Verify(c.header, c.body, t0)
		if !errors.Is(err, ErrInvalidSignature) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestSignatureRejectsReplayOutsideTolerance(t *testing.T) {
	body := []byte(`{}`)
	h := Sign("s", t0, body)
	for name, now := range map[string]time.Time{
		"too old":    t0.Add(DefaultTolerance + time.Second),
		"from ahead": t0.Add(-DefaultTolerance - time.Second),
	} {
		if err := NewVerifier("s").Verify(h, body, now); !errors.Is(err, ErrSignatureTooOld) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if err := NewVerifier("s").Verify(h, body, t0.Add(DefaultTolerance)); err != nil {
		t.Errorf("exactly at the limit is accepted: %v", err)
	}
}

func TestSignatureRejectsMalformedHeaders(t *testing.T) {
	for _, h := range []string{"", "garbage", "t=abc,v1=00", "t=1,v1=", "v1=00", "t=1"} {
		if err := NewVerifier("s").Verify(h, nil, t0); !errors.Is(err, ErrMalformedHeader) {
			t.Errorf("%q: got %v", h, err)
		}
	}
}

func TestBackoffDoublesAndCaps(t *testing.T) {
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute}
	for i, w := range want {
		if got := Backoff(i + 1); got != w {
			t.Errorf("attempt %d: %v, want %v", i+1, got, w)
		}
	}
	if Backoff(100) != backoffCap || Backoff(0) != backoffBase || Backoff(-3) != backoffBase {
		t.Fatal("backoff must cap at six hours and treat bad input as the first attempt")
	}
	for n := 1; n < 40; n++ {
		if Backoff(n+1) < Backoff(n) {
			t.Fatalf("backoff must never shrink: %d", n)
		}
	}
}

func TestAddressPolicyBlocksInternalNetworks(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "10.1.2.3", "172.16.0.1", "172.31.255.255", "192.168.1.1",
		"169.254.169.254", "100.64.0.1", "0.0.0.0", "224.0.0.1", "255.255.255.255",
		"::1", "fe80::1", "fc00::1", "fd12:3456::1", "::ffff:127.0.0.1", "::ffff:10.0.0.1",
		"64:ff9b::7f00:1", "198.18.0.1",
	}
	allowed := []string{"8.8.8.8", "1.1.1.1", "41.158.0.1", "2606:4700:4700::1111", "172.32.0.1"}
	var p Policy
	for _, s := range blocked {
		if p.AllowedAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s must be blocked", s)
		}
	}
	for _, s := range allowed {
		if !p.AllowedAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s must be allowed", s)
		}
	}
	if !(Policy{AllowPrivate: true}).AllowedAddr(netip.MustParseAddr("127.0.0.1")) {
		t.Error("development policy must allow loopback")
	}
}

func TestURLValidation(t *testing.T) {
	var prod Policy
	good := []string{"https://example.com/hooks", "https://shop.example.co.ga:8443/a?b=c"}
	bad := []string{
		"", "http://example.com/h", "ftp://example.com", "example.com/h", "https://",
		"https://user:pw@example.com/h", "https://localhost/h", "https://intranet/h",
		"https://box.local/h", "https://svc.internal/h", "https://127.0.0.1/h",
		"https://169.254.169.254/latest/meta-data", "https://[::1]/h", "https://10.0.0.5:8080/h",
		"https://" + strings.Repeat("a", 2100) + ".com",
	}
	for _, u := range good {
		if err := prod.ValidateURL(u); err != nil {
			t.Errorf("%s: %v", u, err)
		}
	}
	for _, u := range bad {
		if err := prod.ValidateURL(u); !errors.Is(err, ErrUnsafeURL) {
			t.Errorf("%.40s: got %v", u, err)
		}
	}
	dev := Policy{AllowPrivate: true, AllowHTTP: true}
	if err := dev.ValidateURL("http://127.0.0.1:9000/h"); err != nil {
		t.Errorf("development policy: %v", err)
	}
}

// shiftTime returns the header with its timestamp moved one second.
func shiftTime(h string) string {
	old := "t=" + strconv.FormatInt(t0.Unix(), 10)
	return strings.Replace(h, old, "t="+strconv.FormatInt(t0.Unix()+1, 10), 1)
}
