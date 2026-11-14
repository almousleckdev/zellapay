// Command worker runs ZellaPay background jobs.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/zellapay/zellapay/internal/platform/config"
	"github.com/zellapay/zellapay/internal/platform/logging"
)

func main() {
	cfg, err := config.Load("worker", ":8070")
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	log := logging.New(cfg.Service, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("worker started")
	// Job runners register here: polling, resolution, payouts, webhooks.
	<-ctx.Done()
	log.Info("worker stopped")
}
