package webhooks

import (
	"fmt"
	"net/netip"
	"net/url"
	"strings"
)

const maxURLLength = 2048

// Policy decides which destinations webhooks may reach. Merchants supply the
// URLs, so a careless policy would let them probe our internal network
// (server-side request forgery). Production must use the zero value.
type Policy struct {
	AllowPrivate bool // permit loopback and private addresses (development only)
	AllowHTTP    bool // permit plain http (development only)
}

var blockedPrefixes = parsePrefixes(
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.168.0.0/16",
	"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4",
	"240.0.0.0/4", "::/128", "::1/128", "64:ff9b::/96", "100::/64",
	"2001:db8::/32", "fc00::/7", "fe80::/10", "ff00::/8",
)

func parsePrefixes(list ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(list))
	for i, s := range list {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}

// AllowedAddr reports whether a connection to ip is permitted. It is checked
// again at connect time, on the address actually dialled, so a hostname that
// resolves to an internal address after validation is still refused.
func (p Policy) AllowedAddr(ip netip.Addr) bool {
	if p.AllowPrivate {
		return true
	}
	ip = ip.Unmap()
	if !ip.IsValid() {
		return false
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

// ValidateURL checks a merchant-supplied endpoint URL.
func (p Policy) ValidateURL(raw string) error {
	if raw == "" || len(raw) > maxURLLength {
		return fmt.Errorf("%w: length", ErrUnsafeURL)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil {
		return fmt.Errorf("%w: not a plain URL", ErrUnsafeURL)
	}
	schemeOK := u.Scheme == "https" || (p.AllowHTTP && u.Scheme == "http")
	if !schemeOK {
		return fmt.Errorf("%w: https is required", ErrUnsafeURL)
	}
	return p.checkHost(u.Hostname())
}

func (p Policy) checkHost(host string) error {
	if ip, err := netip.ParseAddr(host); err == nil {
		if !p.AllowedAddr(ip) {
			return fmt.Errorf("%w: address not allowed", ErrUnsafeURL)
		}
		return nil
	}
	if p.AllowPrivate {
		return nil
	}
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if !strings.Contains(h, ".") || internalSuffix(h) {
		return fmt.Errorf("%w: host is not public", ErrUnsafeURL)
	}
	return nil
}

func internalSuffix(h string) bool {
	for _, s := range internalSuffixes {
		if strings.HasSuffix(h, s) {
			return true
		}
	}
	return false
}

var internalSuffixes = []string{
	".localhost", ".local", ".internal", ".localdomain", ".lan", ".home",
}
