package idempotency

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/platform/httpx"
)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC) }

// harness wires the middleware around a handler that counts executions.
type harness struct {
	store *memStore
	calls atomic.Int64
	mw    http.Handler
}

type scopeFunc = func(*http.Request) (string, bool)

func merchantScope(*http.Request) (string, bool) { return "m1", true }

func newHarness(t *testing.T, inner func(*harness, http.ResponseWriter, *http.Request)) *harness {
	t.Helper()
	return newHarnessScope(t, merchantScope, inner)
}

func newHarnessScope(
	t *testing.T, scope scopeFunc, inner func(*harness, http.ResponseWriter, *http.Request),
) *harness {
	t.Helper()
	h := &harness{store: newMemStore()}
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.calls.Add(1)
		inner(h, w, r)
	})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	opts := Options{Scope: scope, MaxBody: 1024, Log: log}
	h.mw = httpx.Middleware(log, Middleware(h.store, fixedClock{}, opts)(final))
	return h
}

func created(h *harness, w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(`{"n":` + strconv.FormatInt(h.calls.Load(), 10) + `}`))
}

func (h *harness) post(body, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost, "/v1/collections", strings.NewReader(body))
	if key != "" {
		req.Header.Set(HeaderKey, key)
	}
	rec := httptest.NewRecorder()
	h.mw.ServeHTTP(rec, req)
	return rec
}

func problemCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var b httpx.ErrorBody
	if err := json.NewDecoder(rec.Body).Decode(&b); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	return b.Error.Code
}
