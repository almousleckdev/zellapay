// Command worker runs ZellaPay background jobs for remote operators.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/zellapay/zellapay/internal/platform/config"
	"github.com/zellapay/zellapay/internal/platform/logging"
	"github.com/zellapay/zellapay/internal/wiring"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("worker", ":8070")
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

	log.Info("worker started")
	c.Run(ctx, log)
	log.Info("worker stopped")
	return nil
}
