// Command operator-gateway is the ZellaPay operator-gateway binary.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/zellapay/zellapay/internal/platform/app"
	"github.com/zellapay/zellapay/internal/platform/config"
	"github.com/zellapay/zellapay/internal/platform/httpx"
	"github.com/zellapay/zellapay/internal/platform/logging"
)

func main() {
	if err := run(); err != nil {
		slog.Error("operator-gateway stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("operator-gateway", ":8090")
	if err != nil {
		return err
	}
	log := logging.New(cfg.Service, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	drain := httpx.NewDrain()
	srv := app.NewServer(cfg, newHandler(log, cfg, drain))
	return app.Serve(ctx, log, srv, app.Plan{
		Timeout: cfg.ShutdownTimeout, DrainDelay: cfg.DrainDelay, Drain: drain,
	})
}

func newHandler(log *slog.Logger, cfg config.Config, drain *httpx.Drain) http.Handler {
	mux := http.NewServeMux()
	httpx.Health(mux, drain.Check(httpx.ReadyTimeout, nil))
	return httpx.Harden(log, mux, httpx.BodyLimits{Default: cfg.Server.MaxBodyBytes})
}
