package wiring

import (
	"strings"
	"testing"
)

func TestLocalDashboardDefaultsToTheDevelopmentApp(t *testing.T) {
	setEnv(t, valid())
	s, err := SettingsFromEnv()
	if err != nil || s.DashboardURL != "http://localhost:3001" || s.PasswordPepperVersion != 1 {
		t.Fatalf("got %+v, %v", s, err)
	}
}

func TestIdentitySettingsAreStrictAndNeverEchoSecrets(t *testing.T) {
	for name, mutate := range map[string]func(map[string]string){
		"no pepper":      func(e map[string]string) { delete(e, "ZP_PASSWORD_PEPPER") },
		"short pepper":   func(e map[string]string) { e["ZP_PASSWORD_PEPPER"] = key(8) },
		"bad version":    func(e map[string]string) { e["ZP_PASSWORD_PEPPER_VERSION"] = "0" },
		"http in prod":   func(e map[string]string) { e["ZP_DASHBOARD_URL"] = "http://m.example" },
		"no url in prod": func(e map[string]string) { delete(e, "ZP_DASHBOARD_URL") },
	} {
		env := productionMail()
		mutate(env)
		setEnv(t, env)
		_, err := SettingsFromEnv()
		if err == nil || !strings.Contains(err.Error(), "ZP_") {
			t.Errorf("%s must be refused, got %v", name, err)
		}
	}
}
