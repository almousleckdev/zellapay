// Package wiring builds the application's services from configuration.
// It is the only place that knows how modules fit together.
package wiring

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/webhooks"
)

const minSecretBytes = 32

// Settings are the values the services need. Secrets come from the
// environment here and from the secret manager in production.
type Settings struct {
	Environment string
	Mode        merchants.Mode

	DatabaseURL string
	RedisURL    string

	DataKeyID     string
	DataKey       []byte
	BlindIndexKey []byte

	SimulatorEnabled bool
	SimulatorSecret  []byte
	SimulatorPrefix  string

	// WebhookPolicy limits where webhooks may be sent. Outside production it
	// permits local receivers; production always uses the strict zero value.
	WebhookPolicy webhooks.Policy

	// SettlementDelay is how long collected funds wait before a merchant can
	// pay them out. PayoutMaxAmount caps a single payout, in the smallest unit.
	SettlementDelay time.Duration
	PayoutMaxAmount int64

	// Mail. Outside production, email goes to a local MailDev and SMS is
	// written to the log. Production needs a real mail server, always with
	// TLS, and has no SMS gateway until one is chosen.
	SMTPAddr       string
	MailFrom       string
	SMTPUsername   string
	SMTPPassword   string
	SMTPRequireTLS bool
	SMSLogOnly     bool
}

// SettingsFromEnv reads and validates the settings. The error names the
// offending variable and never includes a secret value.
func SettingsFromEnv() (Settings, error) {
	s := Settings{
		Environment: getenv("ZP_ENV", "local"),
		DatabaseURL: os.Getenv("ZP_DATABASE_URL"),
		RedisURL:    os.Getenv("ZP_REDIS_URL"),
		DataKeyID:   getenv("ZP_DATA_KEY_ID", "k1"),
	}
	s.Mode = merchants.ModeTest
	if s.Environment == "production" {
		s.Mode = merchants.ModeLive
	}
	var errs []error
	errs = append(errs,
		require("ZP_DATABASE_URL", s.DatabaseURL),
		require("ZP_REDIS_URL", s.RedisURL))
	var err error
	s.DataKey, err = secretFromEnv("ZP_DATA_KEY")
	errs = append(errs, err)
	s.BlindIndexKey, err = secretFromEnv("ZP_BLIND_INDEX_KEY")
	errs = append(errs, err)
	errs = append(errs, s.loadSimulator(), s.loadWebhookPolicy(), s.loadLimits(), s.loadMail())
	if err := errors.Join(errs...); err != nil {
		return Settings{}, fmt.Errorf("invalid configuration: %w", err)
	}
	return s, nil
}

func (s *Settings) loadSimulator() error {
	def := strconv.FormatBool(s.Environment != "production")
	on, err := strconv.ParseBool(getenv("ZP_SIMULATOR_ENABLED", def))
	if err != nil {
		return errors.New("ZP_SIMULATOR_ENABLED: not a boolean")
	}
	if !on {
		return nil
	}
	if s.Environment == "production" {
		return errors.New("ZP_SIMULATOR_ENABLED: the simulator must never run in production")
	}
	s.SimulatorEnabled = true
	s.SimulatorPrefix = getenv("ZP_SIMULATOR_PREFIX", "+241")
	s.SimulatorSecret, err = secretFromEnv("ZP_SIMULATOR_SECRET")
	return err
}

// Validate rejects settings that would make the system misbehave silently,
// such as a payout cap of zero that refuses every payout. SettingsFromEnv
// results always pass; code that builds Settings by hand must too.
func (s Settings) Validate() error {
	var errs []error
	if s.PayoutMaxAmount <= 0 {
		errs = append(errs, errors.New("PayoutMaxAmount: must be positive"))
	}
	if s.SettlementDelay < 0 {
		errs = append(errs, errors.New("SettlementDelay: must not be negative"))
	}
	if s.SMTPAddr == "" || s.MailFrom == "" {
		errs = append(errs, errors.New("SMTPAddr and MailFrom: required"))
	}
	return errors.Join(errs...)
}

func (s *Settings) loadMail() error {
	prod := s.Environment == "production"
	s.SMTPAddr = os.Getenv("ZP_SMTP_ADDR")
	s.MailFrom = os.Getenv("ZP_MAIL_FROM")
	s.SMTPUsername = os.Getenv("ZP_SMTP_USERNAME")
	s.SMTPPassword = os.Getenv("ZP_SMTP_PASSWORD")
	s.SMSLogOnly = !prod
	if !prod {
		s.SMTPAddr = getenv("ZP_SMTP_ADDR", "127.0.0.1:1026")
		s.MailFrom = getenv("ZP_MAIL_FROM", "ZellaPay <no-reply@zellapay.test>")
	}
	tls, err := strconv.ParseBool(getenv("ZP_SMTP_REQUIRE_TLS", strconv.FormatBool(prod)))
	if err != nil {
		return errors.New("ZP_SMTP_REQUIRE_TLS: not a boolean")
	}
	s.SMTPRequireTLS = tls
	if !prod {
		return nil
	}
	return errors.Join(
		require("ZP_SMTP_ADDR", s.SMTPAddr),
		require("ZP_MAIL_FROM", s.MailFrom),
		requireTrue("ZP_SMTP_REQUIRE_TLS", tls, "production email must use TLS"),
	)
}

func requireTrue(name string, ok bool, why string) error {
	if ok {
		return nil
	}
	return fmt.Errorf("%s: %s", name, why)
}

func (s *Settings) loadLimits() error {
	d, err := time.ParseDuration(getenv("ZP_SETTLEMENT_DELAY", "24h"))
	if err != nil || d < 0 {
		return errors.New("ZP_SETTLEMENT_DELAY: not a valid duration (for example 24h)")
	}
	s.SettlementDelay = d
	max, err := strconv.ParseInt(getenv("ZP_PAYOUT_MAX_AMOUNT", "5000000"), 10, 64)
	if err != nil || max <= 0 {
		return errors.New("ZP_PAYOUT_MAX_AMOUNT: must be a positive whole number")
	}
	s.PayoutMaxAmount = max
	return nil
}

func (s *Settings) loadWebhookPolicy() error {
	def := strconv.FormatBool(s.Environment != "production")
	allow, err := strconv.ParseBool(getenv("ZP_WEBHOOKS_ALLOW_PRIVATE", def))
	if err != nil {
		return errors.New("ZP_WEBHOOKS_ALLOW_PRIVATE: not a boolean")
	}
	if allow && s.Environment == "production" {
		return errors.New("ZP_WEBHOOKS_ALLOW_PRIVATE: never allowed in production")
	}
	s.WebhookPolicy = webhooks.Policy{AllowPrivate: allow, AllowHTTP: allow}
	return nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func require(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s: required", name)
	}
	return nil
}

// secretFromEnv decodes a base64 secret of at least 32 bytes.
func secretFromEnv(name string) ([]byte, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return nil, fmt.Errorf("%s: required", name)
	}
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: not valid base64", name)
	}
	if len(b) < minSecretBytes {
		return nil, fmt.Errorf("%s: must decode to at least %d bytes", name, minSecretBytes)
	}
	return b, nil
}
