package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	c, err := Load("api", ":8080")
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" || c.Env != "local" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestLoadBadDuration(t *testing.T) {
	t.Setenv("ZP_SHUTDOWN_TIMEOUT", "soon")
	if _, err := Load("api", ":8080"); err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestDocsEnabledByEnvironment(t *testing.T) {
	cases := []struct {
		env, override string
		want          bool
	}{
		{"local", "", true},
		{"staging", "", true},
		{"production", "", false},
		{"production", "true", true},
		{"local", "false", false},
	}
	for _, c := range cases {
		t.Setenv("ZP_ENV", c.env)
		t.Setenv("ZP_DOCS_ENABLED", c.override)
		cfg, err := Load("api", ":8080")
		if err != nil || cfg.DocsEnabled != c.want {
			t.Errorf("%+v: got %v, %v", c, cfg.DocsEnabled, err)
		}
	}
}

func TestLoadRejectsBadDocsFlag(t *testing.T) {
	t.Setenv("ZP_DOCS_ENABLED", "maybe")
	if _, err := Load("api", ":8080"); err == nil {
		t.Fatal("expected error")
	}
}
