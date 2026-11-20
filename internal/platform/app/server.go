package app

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/platform/config"
)

// NewServer builds the http.Server every binary uses, so the timeouts are
// defined once.
func NewServer(cfg config.Config, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           h,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}
}
