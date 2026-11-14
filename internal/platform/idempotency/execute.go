package idempotency

import (
	"context"
	"errors"
	"net/http"

	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// execute runs the real handler and then stores or releases the key. A
// panic releases the key before it propagates, so the client can retry.
func (h *handler) execute(w http.ResponseWriter, r *http.Request, req Request, body []byte) {
	restoreBody(r, body)
	rec := newRecorder(w, int(h.opts.MaxBody))
	finished := false
	defer func() {
		if !finished {
			h.release(r, req)
		}
	}()
	h.next.ServeHTTP(rec, r)
	finished = true
	h.finish(r, req, rec)
}

func (h *handler) finish(r *http.Request, req Request, rec *recorder) {
	if !rec.replayable() {
		h.release(r, req)
		return
	}
	ctx := context.WithoutCancel(r.Context())
	err := h.store.Complete(ctx, req, rec.response(), h.clock.Now())
	switch {
	case err == nil:
	case errors.Is(err, ErrLockLost):
		h.opts.Log.Warn("idempotency lock lost",
			"request_id", httpx.RequestID(r.Context()))
	default:
		h.opts.Log.Error("idempotency complete failed", "err", err,
			"request_id", httpx.RequestID(r.Context()))
	}
}

func (h *handler) release(r *http.Request, req Request) {
	// Detached: the client may have disconnected, but the key must still
	// be freed.
	if err := h.store.Release(context.WithoutCancel(r.Context()), req); err != nil {
		h.opts.Log.Error("idempotency release failed", "err", err,
			"request_id", httpx.RequestID(r.Context()))
	}
}
