package wiring

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
)

// loopback is trusted by default outside production, so a dashboard running
// on the same machine can report its users' addresses.
var loopback = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("::1/128"),
}

// loadTrustedProxies reads ZP_TRUSTED_PROXIES, a comma-separated list of CIDR
// ranges whose X-Forwarded-For header is believed. Production trusts nobody
// until the load balancer's range is listed.
func (s *Settings) loadTrustedProxies() error {
	raw := strings.TrimSpace(os.Getenv("ZP_TRUSTED_PROXIES"))
	if raw == "" {
		if s.Environment != "production" {
			s.TrustedProxies = loopback
		}
		return nil
	}
	for _, part := range strings.Split(raw, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(part))
		if err != nil {
			return fmt.Errorf("ZP_TRUSTED_PROXIES: %q is not a CIDR range", strings.TrimSpace(part))
		}
		if prefix.Bits() == 0 {
			return errors.New("ZP_TRUSTED_PROXIES: trusting every address lets anyone spoof theirs")
		}
		s.TrustedProxies = append(s.TrustedProxies, prefix.Masked())
	}
	return nil
}
