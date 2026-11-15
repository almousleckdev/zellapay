package wiring

import (
	"strings"
	"testing"
)

func TestLocalTrustsLoopbackProxiesAndProductionTrustsNone(t *testing.T) {
	setEnv(t, valid())
	local, err := SettingsFromEnv()
	if err != nil || len(local.TrustedProxies) != 2 {
		t.Fatalf("local: %v, %v", local.TrustedProxies, err)
	}
	setEnv(t, productionMail())
	prod, err := SettingsFromEnv()
	if err != nil || len(prod.TrustedProxies) != 0 {
		t.Fatalf("production: %v, %v", prod.TrustedProxies, err)
	}
}

func TestTrustedProxiesAreParsedAndStrict(t *testing.T) {
	env := valid()
	env["ZP_TRUSTED_PROXIES"] = "10.0.0.0/8, 172.16.1.7/32"
	setEnv(t, env)
	s, err := SettingsFromEnv()
	if err != nil || len(s.TrustedProxies) != 2 || s.TrustedProxies[1].String() != "172.16.1.7/32" {
		t.Fatalf("got %v, %v", s.TrustedProxies, err)
	}
	for name, bad := range map[string]string{
		"not a range":     "10.0.0.1",
		"everything":      "0.0.0.0/0",
		"everything IPv6": "::/0",
	} {
		env["ZP_TRUSTED_PROXIES"] = bad
		setEnv(t, env)
		_, err := SettingsFromEnv()
		if err == nil || !strings.Contains(err.Error(), "ZP_TRUSTED_PROXIES") {
			t.Errorf("%s must be refused, got %v", name, err)
		}
	}
}
