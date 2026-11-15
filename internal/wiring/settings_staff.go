package wiring

import (
	"errors"
	"net/url"
	"strings"
)

// loadStaff reads where the admin console lives, which passkeys are bound to.
// A passkey works only on the site that created it, so these must match the
// address staff really use.
func (s *Settings) loadStaff() error {
	prod := s.Environment == "production"
	def := "http://localhost:3002"
	if prod {
		def = ""
	}
	s.AdminURL = strings.TrimRight(getenv("ZP_ADMIN_URL", def), "/")
	u, err := url.Parse(s.AdminURL)
	if err != nil || u.Host == "" || (prod && u.Scheme != "https") {
		return errors.New("ZP_ADMIN_URL: a full address is required, https in production")
	}
	s.WebAuthnRPID = getenv("ZP_WEBAUTHN_RP_ID", u.Hostname())
	return nil
}
