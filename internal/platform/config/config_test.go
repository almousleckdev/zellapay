package config

import (
	"testing"
	"time"
)

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

func TestLoadServerDefaultsPreserveCurrentValues(t *testing.T) {
	c, err := Load("api", ":8080")
	if err != nil {
		t.Fatal(err)
	}
	want := Server{
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxBodyBytes:      1 << 20,
	}
	if c.Server != want {
		t.Fatalf("got %+v, want %+v", c.Server, want)
	}
	if c.ShutdownTimeout != 20*time.Second || c.DrainDelay != 0 {
		t.Fatalf("shutdown %v, drain %v", c.ShutdownTimeout, c.DrainDelay)
	}
}

func TestLoadDrainDelay(t *testing.T) {
	t.Setenv("ZP_DRAIN_DELAY", "7s")
	c, err := Load("api", ":8080")
	if err != nil || c.DrainDelay != 7*time.Second {
		t.Fatalf("got %v, %v", c.DrainDelay, err)
	}
}

func TestLoadBadDrainDelay(t *testing.T) {
	t.Setenv("ZP_DRAIN_DELAY", "soon")
	if _, err := Load("api", ":8080"); err == nil {
		t.Fatal("expected error for invalid duration")
	}
}
