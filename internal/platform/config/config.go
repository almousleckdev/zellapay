// Package config loads service configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds settings shared by every binary.
type Config struct {
	Service         string
	Env             string // local | test | staging | production
	HTTPAddr        string
	LogLevel        string
	DocsEnabled     bool // serve Swagger UI and the OpenAPI files
	ShutdownTimeout time.Duration
}

// Load reads configuration for the named service.
// Defaults suit local development.
func Load(service, defaultAddr string) (Config, error) {
	c := Config{
		Service:  service,
		Env:      get("ZP_ENV", "local"),
		HTTPAddr: get("ZP_HTTP_ADDR", defaultAddr),
		LogLevel: get("ZP_LOG_LEVEL", "info"),
	}
	d, err := time.ParseDuration(get("ZP_SHUTDOWN_TIMEOUT", "20s"))
	if err != nil {
		return Config{}, fmt.Errorf("ZP_SHUTDOWN_TIMEOUT: %w", err)
	}
	c.ShutdownTimeout = d
	// Interactive docs are on everywhere except production unless asked.
	defaultDocs := strconv.FormatBool(c.Env != "production")
	docs, err := strconv.ParseBool(get("ZP_DOCS_ENABLED", defaultDocs))
	if err != nil {
		return Config{}, fmt.Errorf("ZP_DOCS_ENABLED: %w", err)
	}
	c.DocsEnabled = docs
	return c, nil
}

func get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
