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
	DrainDelay      time.Duration
	Server          Server
}

// Server holds the limits applied to every HTTP server.
type Server struct {
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxBodyBytes      int64
}

func defaultServer() Server {
	return Server{
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxBodyBytes:      1 << 20,
	}
}

// Load reads configuration for the named service.
// Defaults suit local development.
func Load(service, defaultAddr string) (Config, error) {
	c := Config{
		Service:  service,
		Env:      get("ZP_ENV", "local"),
		HTTPAddr: get("ZP_HTTP_ADDR", defaultAddr),
		LogLevel: get("ZP_LOG_LEVEL", "info"),
		Server:   defaultServer(),
	}
	var err error
	if c.ShutdownTimeout, err = duration("ZP_SHUTDOWN_TIMEOUT", "20s"); err != nil {
		return Config{}, err
	}
	if c.DrainDelay, err = duration("ZP_DRAIN_DELAY", "0s"); err != nil {
		return Config{}, err
	}
	// Interactive docs are on everywhere except production unless asked.
	defaultDocs := strconv.FormatBool(c.Env != "production")
	docs, err := strconv.ParseBool(get("ZP_DOCS_ENABLED", defaultDocs))
	if err != nil {
		return Config{}, fmt.Errorf("ZP_DOCS_ENABLED: %w", err)
	}
	c.DocsEnabled = docs
	return c, nil
}

func duration(key, def string) (time.Duration, error) {
	d, err := time.ParseDuration(get(key, def))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}

func get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
