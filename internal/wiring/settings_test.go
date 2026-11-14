package wiring

import (
	"encoding/base64"
	"strings"
	"testing"
)

func key(n int) string { return base64.StdEncoding.EncodeToString(make([]byte, n)) }

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for _, name := range []string{
		"ZP_ENV", "ZP_DATABASE_URL", "ZP_REDIS_URL", "ZP_DATA_KEY", "ZP_DATA_KEY_ID",
		"ZP_BLIND_INDEX_KEY", "ZP_SIMULATOR_ENABLED", "ZP_SIMULATOR_SECRET", "ZP_SIMULATOR_PREFIX",
		"ZP_WEBHOOKS_ALLOW_PRIVATE",
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
	env := valid()
	env["ZP_ENV"] = "production"
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
	env := valid()
	env["ZP_ENV"] = "production"
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
