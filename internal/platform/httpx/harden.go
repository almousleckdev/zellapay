package httpx

import (
	"log/slog"
	"net/http"
	"time"
)

// ReadyTimeout bounds each readiness probe.
const ReadyTimeout = 2 * time.Second

// Harden wraps mux with the middleware every binary shares. From the
// outside in: request ID and access log, panic recovery, body limits.
func Harden(log *slog.Logger, mux *http.ServeMux, limits BodyLimits) http.Handler {
	return Middleware(log, Recover(log)(limits.Wrap(mux)))
}
