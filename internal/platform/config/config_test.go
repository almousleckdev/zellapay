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
