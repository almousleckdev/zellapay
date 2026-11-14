package idempotency

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func respondWith(status int) func(*harness, http.ResponseWriter, *http.Request) {
	return func(_ *harness, w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{}`))
	}
}

func TestResponsesThatMustBeRetriedAreNotStored(t *testing.T) {
	for _, status := range []int{500, 502, 503, 429, 408} {
		h := newHarness(t, respondWith(status))
		h.post("{}", "k1")
		h.post("{}", "k1")
		if h.calls.Load() != 2 || h.store.completes != 0 {
			t.Errorf("status %d: calls=%d completes=%d", status, h.calls.Load(), h.store.completes)
		}
	}
}

func TestDeterministicErrorsAreStoredAndReplayed(t *testing.T) {
	for _, status := range []int{200, 201, 400, 402, 404, 422} {
		h := newHarness(t, respondWith(status))
		h.post("{}", "k1")
		replay := h.post("{}", "k1")
		if h.calls.Load() != 1 || replay.Code != status {
			t.Errorf("status %d: calls=%d replay=%d", status, h.calls.Load(), replay.Code)
		}
	}
}

func TestOversizedResponseIsNotStored(t *testing.T) {
	h := newHarness(t, func(_ *harness, w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 4096)))
	})
	h.post("{}", "k1")
	h.post("{}", "k1")
	if h.calls.Load() != 2 {
		t.Fatalf("handler ran %d times, want 2", h.calls.Load())
	}
}

func TestPanicReleasesTheKey(t *testing.T) {
	boom := true
	h := newHarness(t, func(h *harness, w http.ResponseWriter, r *http.Request) {
		if boom {
			panic("handler failed")
		}
		created(h, w, r)
	})
	func() {
		defer func() { _ = recover() }()
		h.post("{}", "k1")
	}()
	boom = false
	if rec := h.post("{}", "k1"); rec.Code != http.StatusCreated {
		t.Fatalf("retry after panic got %d", rec.Code)
	}
	if h.store.releases != 1 {
		t.Fatalf("releases = %d, want 1", h.store.releases)
	}
}

func TestConcurrentDuplicateIsToldItIsInFlight(t *testing.T) {
	entered, proceed := make(chan struct{}), make(chan struct{})
	h := newHarness(t, func(h *harness, w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-proceed
		created(h, w, r)
	})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.post("{}", "k1")
	}()
	<-entered
	rec := h.post("{}", "k1")
	close(proceed)
	wg.Wait()
	if rec.Code != http.StatusConflict || problemCode(t, rec) != "idempotency_key_in_use" {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestSameKeyForDifferentScopesIsIndependent(t *testing.T) {
	store := newMemStore()
	var calls int
	final := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusCreated)
	})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, scope := range []string{"m1", "m2"} {
		opts := Options{
			Scope: func(*http.Request) (string, bool) { return scope, true },
			Log:   log,
		}
		h := Middleware(store, fixedClock{}, opts)(final)
		req := httptest.NewRequestWithContext(context.Background(),
			http.MethodPost, "/v1/x", strings.NewReader("{}"))
		req.Header.Set(HeaderKey, "shared-key")
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
	if calls != 2 {
		t.Fatalf("each merchant must execute once, got %d", calls)
	}
}
