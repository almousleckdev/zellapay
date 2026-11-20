// Package app runs an HTTP server with graceful shutdown.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// Plan says how Serve stops. Listener is optional; without it Serve listens
// on the server's address.
type Plan struct {
	Timeout    time.Duration
	DrainDelay time.Duration
	Drain      *httpx.Drain
	Listener   net.Listener
}

// Serve runs srv until ctx is done, then stops in a fixed order: readiness
// starts failing, load balancers get DrainDelay to notice, and in-flight
// requests get Timeout to finish. The caller owns ctx and its signals.
func Serve(ctx context.Context, log *slog.Logger, srv *http.Server, plan Plan) error {
	ln, err := listener(ctx, srv, plan)
	if err != nil {
		return err
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", ln.Addr().String())
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	return stop(log, srv, plan)
}

func listener(ctx context.Context, srv *http.Server, plan Plan) (net.Listener, error) {
	if plan.Listener != nil {
		return plan.Listener, nil
	}
	var lc net.ListenConfig
	return lc.Listen(ctx, "tcp", srv.Addr)
}

func stop(log *slog.Logger, srv *http.Server, plan Plan) error {
	log.Info("draining", "delay", plan.DrainDelay.String())
	plan.Drain.Start()
	time.Sleep(plan.DrainDelay)
	log.Info("shutting down", "timeout", plan.Timeout.String())
	ctx, cancel := context.WithTimeout(context.Background(), plan.Timeout)
	defer cancel()
	return srv.Shutdown(ctx)
}
