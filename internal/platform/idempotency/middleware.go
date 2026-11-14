package idempotency

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"

	"github.com/zellapay/zellapay/internal/platform/clock"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

const (
	// HeaderKey is the request header carrying the client's key.
	HeaderKey = "Idempotency-Key"
	// HeaderReplayed marks a response served from a stored result.
	HeaderReplayed = "Idempotent-Replayed"

	defaultMaxBody = 1 << 20 // 1 MiB, for requests and stored responses
)

// Options configures the middleware.
type Options struct {
	// Scope returns the tenant the request belongs to. It must be set by
	// authentication that ran earlier. Returning false is a wiring error.
	Scope func(*http.Request) (string, bool)
	// MaxBody bounds request and stored response sizes. Zero means 1 MiB.
	MaxBody int64
	Log     *slog.Logger
}

type handler struct {
	next  http.Handler
	store Store
	clock clock.Clock
	opts  Options
}

// Middleware requires an Idempotency-Key on POST requests and makes them
// execute at most once per scope and key. Other methods pass through.
func Middleware(s Store, c clock.Clock, opts Options) func(http.Handler) http.Handler {
	if opts.MaxBody <= 0 {
		opts.MaxBody = defaultMaxBody
	}
	return func(next http.Handler) http.Handler {
		return &handler{next: next, store: s, clock: c, opts: opts}
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.next.ServeHTTP(w, r)
		return
	}
	req, body, ok := h.prepare(w, r)
	if !ok {
		return
	}
	decision, err := h.store.Begin(r.Context(), req, h.clock.Now())
	if err != nil {
		h.fail(w, r, "begin", err)
		return
	}
	switch decision.Kind {
	case Started:
		h.execute(w, r, req, body)
	case Replay:
		writeReplay(w, decision.Response)
	case InFlight:
		httpx.WriteError(w, r, problemKeyInUse)
	default:
		httpx.WriteError(w, r, problemKeyReused)
	}
}

// prepare validates the key and reads the bounded body.
func (h *handler) prepare(w http.ResponseWriter, r *http.Request) (Request, []byte, bool) {
	key := r.Header.Get(HeaderKey)
	if key == "" {
		httpx.WriteError(w, r, problemKeyRequired)
		return Request{}, nil, false
	}
	if validateKey(key) != nil {
		httpx.WriteError(w, r, problemKeyInvalid)
		return Request{}, nil, false
	}
	scope, ok := h.opts.Scope(r)
	if !ok {
		h.fail(w, r, "scope", io.ErrUnexpectedEOF)
		return Request{}, nil, false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, h.opts.MaxBody))
	if err != nil {
		httpx.WriteError(w, r, problemTooLarge)
		return Request{}, nil, false
	}
	return Request{Scope: scope, Key: key, Fingerprint: fingerprint(r, body)}, body, true
}

func (h *handler) fail(w http.ResponseWriter, r *http.Request, step string, err error) {
	h.opts.Log.Error("idempotency", "step", step, "err", err,
		"request_id", httpx.RequestID(r.Context()))
	httpx.WriteError(w, r, problemInternal)
}

func writeReplay(w http.ResponseWriter, resp Response) {
	if resp.ContentType != "" {
		w.Header().Set("Content-Type", resp.ContentType)
	}
	w.Header().Set(HeaderReplayed, "true")
	w.WriteHeader(resp.Status)
	_, _ = w.Write(resp.Body)
}

func restoreBody(r *http.Request, body []byte) {
	r.Body = io.NopCloser(bytes.NewReader(body))
}
