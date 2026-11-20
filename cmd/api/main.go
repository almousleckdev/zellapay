// Command api serves the ZellaPay HTTP API.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/zellapay/zellapay/internal/apiserver"
	"github.com/zellapay/zellapay/internal/merchants/httpauth"
	"github.com/zellapay/zellapay/internal/platform/app"
	"github.com/zellapay/zellapay/internal/platform/config"
	"github.com/zellapay/zellapay/internal/platform/httpx"
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
	stopSimulator := runSimulator(c, log)

	drain := httpx.NewDrain()
	srv := app.NewServer(cfg, apiserver.New(c, apiserver.Options{
		DocsEnabled:  cfg.DocsEnabled,
		ClientIP:     httpauth.Forwarded(c.Settings.TrustedProxies),
		MaxBodyBytes: cfg.Server.MaxBodyBytes,
		Drain:        drain,
	}, log))
	err = app.Serve(ctx, log, srv, app.Plan{
		Timeout: cfg.ShutdownTimeout, DrainDelay: cfg.DrainDelay, Drain: drain,
	})
	stopSimulator()
	return err
}
