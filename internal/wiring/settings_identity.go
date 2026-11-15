package wiring

import (
	"errors"
	"strconv"
	"strings"
)

// loadIdentity reads the password pepper and the dashboard address.
func (s *Settings) loadIdentity() error {
	var errs []error
	var err error
	s.PasswordPepper, err = secretFromEnv("ZP_PASSWORD_PEPPER")
	errs = append(errs, err)
	v, err := strconv.Atoi(getenv("ZP_PASSWORD_PEPPER_VERSION", "1"))
	if err != nil || v < 1 {
		errs = append(errs, errors.New("ZP_PASSWORD_PEPPER_VERSION: must be 1 or more"))
	}
	s.PasswordPepperVersion = v
	prod := s.Environment == "production"
	def := "http://localhost:3001"
	if prod {
		def = ""
	}
	s.DashboardURL = strings.TrimRight(getenv("ZP_DASHBOARD_URL", def), "/")
	if prod && !strings.HasPrefix(s.DashboardURL, "https://") {
		errs = append(errs, errors.New("ZP_DASHBOARD_URL: production links must use https"))
	}
	return errors.Join(errs...)
}
