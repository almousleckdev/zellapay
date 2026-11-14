// Command api is the ZellaPay api binary.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/zellapay/zellapay/internal/apidocs"
	"github.com/zellapay/zellapay/internal/platform/app"
	"github.com/zellapay/zellapay/internal/platform/config"
	"github.com/zellapay/zellapay/internal/platform/httpx"
	"github.com/zellapay/zellapay/internal/platform/logging"
)

func main() {
	cfg, err := config.Load("api", ":8080")
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	log := logging.New(cfg.Service, cfg.LogLevel)

	mux := http.NewServeMux()
	httpx.Health(mux, nil)
	if cfg.DocsEnabled {
		apidocs.Register(mux)
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpx.Middleware(log, mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := app.Serve(log, srv, cfg.ShutdownTimeout); err != nil {
		log.Error("server", "err", err)
		os.Exit(1)
	}
}
