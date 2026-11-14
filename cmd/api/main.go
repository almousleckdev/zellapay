// Command api serves the ZellaPay HTTP API.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zellapay/zellapay/internal/apiserver"
	"github.com/zellapay/zellapay/internal/platform/app"
	"github.com/zellapay/zellapay/internal/platform/config"
	"github.com/zellapay/zellapay/internal/platform/logging"
	"github.com/zellapay/zellapay/internal/wiring"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("api", ":8080")
	if err != nil {
		return err
	}
	settings, err := wiring.SettingsFromEnv()
	if err != nil {
		return err
	}
	log := logging.New(cfg.Service, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	c, err := wiring.Build(ctx, settings, log)
	if err != nil {
		return err
	}
	defer c.Close()
	if c.Simulator != nil {
		go c.Run(ctx, log) // the simulator lives in this process
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           apiserver.New(c, apiserver.Options{DocsEnabled: cfg.DocsEnabled}, log),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return app.Serve(log, srv, cfg.ShutdownTimeout)
}
