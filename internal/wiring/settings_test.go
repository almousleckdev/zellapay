package wiring

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func key(n int) string { return base64.StdEncoding.EncodeToString(make([]byte, n)) }

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for _, name := range []string{
		"ZP_ENV", "ZP_DATABASE_URL", "ZP_REDIS_URL", "ZP_DATA_KEY", "ZP_DATA_KEY_ID",
		"ZP_BLIND_INDEX_KEY", "ZP_SIMULATOR_ENABLED", "ZP_SIMULATOR_SECRET", "ZP_SIMULATOR_PREFIX",
		"ZP_WEBHOOKS_ALLOW_PRIVATE", "ZP_SETTLEMENT_DELAY", "ZP_PAYOUT_MAX_AMOUNT",
		"ZP_SMTP_ADDR", "ZP_MAIL_FROM", "ZP_SMTP_USERNAME", "ZP_SMTP_PASSWORD",
		"ZP_SMTP_REQUIRE_TLS", "ZP_PASSWORD_PEPPER", "ZP_PASSWORD_PEPPER_VERSION",
		"ZP_DASHBOARD_URL",
	} {
		t.Setenv(name, "")
	}
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func valid() map[string]string {
	return map[string]string{
		"ZP_DATABASE_URL": "postgres://zellapay@db.internal/zellapay",
		"ZP_REDIS_URL":    "redis://cache.internal:6379/0",
		"ZP_DATA_KEY":     key(32), "ZP_BLIND_INDEX_KEY": key(32), "ZP_SIMULATOR_SECRET": key(32),
		"ZP_PASSWORD_PEPPER": key(32),
	}
}

func TestLocalDefaultsEnableTheSimulatorInTestMode(t *testing.T) {
	setEnv(t, valid())
	s, err := SettingsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	sandbox := s.SimulatorEnabled && s.Mode == "test" && s.SimulatorPrefix == "+241"
	if !sandbox || s.DataKeyID != "k1" {
		t.Fatalf("unexpected settings %+v", s)
	}
}

func TestProductionIsLiveModeAndRefusesTheSimulator(t *testing.T) {
	env := productionMail()
	setEnv(t, env)
	s, err := SettingsFromEnv()
	if err != nil || s.Mode != "live" || s.SimulatorEnabled {
		t.Fatalf("got %+v, %v", s, err)
	}
	env["ZP_SIMULATOR_ENABLED"] = "true"
	setEnv(t, env)
	_, err = SettingsFromEnv()
	if err == nil || !strings.Contains(err.Error(), "never run in production") {
		t.Fatalf("got %v", err)
	}
}

func TestEveryProblemIsReportedAndNoSecretIsEchoed(t *testing.T) {
	setEnv(t, map[string]string{"ZP_DATA_KEY": "c2hvcnQ=", "ZP_BLIND_INDEX_KEY": "%%%not-base64"})
	_, err := SettingsFromEnv()
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	wanted := []string{"ZP_DATABASE_URL", "ZP_REDIS_URL", "ZP_DATA_KEY", "ZP_BLIND_INDEX_KEY"}
	for _, want := range wanted {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %s in %q", want, msg)
		}
	}
	if strings.Contains(msg, "c2hvcnQ") || strings.Contains(msg, "not-base64") {
		t.Fatalf("the error leaks a secret value: %q", msg)
	}
}

func TestShortSecretsAreRejected(t *testing.T) {
	env := valid()
	env["ZP_DATA_KEY"] = key(16)
	setEnv(t, env)
	if _, err := SettingsFromEnv(); err == nil || !strings.Contains(err.Error(), "ZP_DATA_KEY") {
		t.Fatalf("got %v", err)
	}
}

func TestWebhookDestinationsAreStrictInProduction(t *testing.T) {
	env := productionMail()
	setEnv(t, env)
	s, err := SettingsFromEnv()
	if err != nil || s.WebhookPolicy.AllowPrivate || s.WebhookPolicy.AllowHTTP {
		t.Fatalf("production must be strict: %+v, %v", s.WebhookPolicy, err)
	}
	env["ZP_WEBHOOKS_ALLOW_PRIVATE"] = "true"
	setEnv(t, env)
	_, err = SettingsFromEnv()
	if err == nil || !strings.Contains(err.Error(), "ZP_WEBHOOKS_ALLOW_PRIVATE") {
		t.Fatalf("got %v", err)
	}
}

func TestWebhookDestinationsAreOpenForLocalDevelopment(t *testing.T) {
	setEnv(t, valid())
	s, err := SettingsFromEnv()
	if err != nil || !s.WebhookPolicy.AllowPrivate {
		t.Fatalf("got %+v, %v", s.WebhookPolicy, err)
	}
}

func TestLimitsDefaultToSafeValuesAndAreValidated(t *testing.T) {
	setEnv(t, valid())
	s, err := SettingsFromEnv()
	if err != nil || s.SettlementDelay != 24*time.Hour || s.PayoutMaxAmount != 5_000_000 {
		t.Fatalf("defaults: %+v, %v", s, err)
	}
	env := valid()
	env["ZP_SETTLEMENT_DELAY"], env["ZP_PAYOUT_MAX_AMOUNT"] = "12h", "250000"
	setEnv(t, env)
	s, err = SettingsFromEnv()
	if err != nil || s.SettlementDelay != 12*time.Hour || s.PayoutMaxAmount != 250_000 {
		t.Fatalf("overrides: %+v, %v", s, err)
	}
	invalid := map[string]string{"ZP_SETTLEMENT_DELAY": "soon", "ZP_PAYOUT_MAX_AMOUNT": "0"}
	for name, bad := range invalid {
		env := valid()
		env[name] = bad
		setEnv(t, env)
		if _, err := SettingsFromEnv(); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s=%s: got %v", name, bad, err)
		}
	}
}

func TestHandBuiltSettingsCannotSilentlyRefuseEveryPayout(t *testing.T) {
	base := Settings{
		PayoutMaxAmount: 1, SMTPAddr: "h:25", MailFrom: "a@b.com",
		PasswordPepper: make([]byte, 32), DashboardURL: "http://localhost:3001",
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("an immediate settlement is allowed: %v", err)
	}
	with := func(change func(*Settings)) Settings {
		s := base
		change(&s)
		return s
	}
	for name, s := range map[string]Settings{
		"zero cap":       with(func(s *Settings) { s.PayoutMaxAmount = 0 }),
		"negative cap":   with(func(s *Settings) { s.PayoutMaxAmount = -5 }),
		"negative delay": with(func(s *Settings) { s.SettlementDelay = -time.Second }),
		"no mail server": with(func(s *Settings) { s.SMTPAddr = "" }),
		"no mail sender": with(func(s *Settings) { s.MailFrom = "" }),
		"short pepper":   with(func(s *Settings) { s.PasswordPepper = []byte("x") }),
		"no dashboard":   with(func(s *Settings) { s.DashboardURL = "" }),
	} {
		if err := s.Validate(); err == nil {
			t.Errorf("%s must be rejected", name)
		}
	}
}

func TestDevelopmentMailGoesToMailDevAndSMSToTheLog(t *testing.T) {
	setEnv(t, valid())
	s, err := SettingsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if s.SMTPAddr != "127.0.0.1:1026" || !strings.Contains(s.MailFrom, "zellapay.test") ||
		s.SMTPRequireTLS || !s.SMSLogOnly {
		t.Fatalf("unexpected development mail settings: %+v", s)
	}
}

func productionMail() map[string]string {
	env := valid()
	env["ZP_ENV"] = "production"
	env["ZP_DASHBOARD_URL"] = "https://merchant.zellapay.example"
	env["ZP_SMTP_ADDR"] = "smtp.mail.example:587"
	env["ZP_MAIL_FROM"] = "ZellaPay <no-reply@zellapay.example>"
	return env
}

func TestProductionMailNeedsARealServerAndAlwaysUsesTLS(t *testing.T) {
	setEnv(t, productionMail())
	s, err := SettingsFromEnv()
	if err != nil || !s.SMTPRequireTLS || s.SMSLogOnly {
		t.Fatalf("a valid production setup must pass: %+v, %v", s, err)
	}
	for name, mutate := range map[string]func(map[string]string){
		"no server":      func(e map[string]string) { delete(e, "ZP_SMTP_ADDR") },
		"no sender":      func(e map[string]string) { delete(e, "ZP_MAIL_FROM") },
		"TLS turned off": func(e map[string]string) { e["ZP_SMTP_REQUIRE_TLS"] = "false" },
	} {
		env := productionMail()
		mutate(env)
		setEnv(t, env)
		if _, err := SettingsFromEnv(); err == nil || !strings.Contains(err.Error(), "ZP_") {
			t.Errorf("%s must be refused, got %v", name, err)
		}
	}
}

func TestProductionNeverFallsBackToTheDevelopmentMailServer(t *testing.T) {
	env := valid()
	env["ZP_ENV"] = "production"
	env["ZP_DASHBOARD_URL"] = "https://merchant.zellapay.example"
	setEnv(t, env)
	_, err := SettingsFromEnv()
	if err == nil || !strings.Contains(err.Error(), "ZP_SMTP_ADDR") {
		t.Fatalf("production must not default to MailDev, got %v", err)
	}
}

func TestMailPasswordsAreNeverEchoedInErrors(t *testing.T) {
	env := productionMail()
	env["ZP_SMTP_PASSWORD"] = "hunter2-very-secret"
	env["ZP_SMTP_REQUIRE_TLS"] = "false"
	setEnv(t, env)
	if _, err := SettingsFromEnv(); err == nil || strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("got %v", err)
	}
}
