package idempotency

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFirstRequestRunsAndRetryReplays(t *testing.T) {
	h := newHarness(t, created)
	first := h.post(`{"a":1}`, "k1")
	second := h.post(`{"a":1}`, "k1")

	if h.calls.Load() != 1 {
		t.Fatalf("handler ran %d times, want 1", h.calls.Load())
	}
	if second.Code != http.StatusCreated || second.Body.String() != first.Body.String() {
		t.Fatalf("replay differs: %d %q vs %q", second.Code, second.Body, first.Body)
	}
	if second.Header().Get(HeaderReplayed) != "true" || first.Header().Get(HeaderReplayed) != "" {
		t.Fatal("only the replay carries the replayed header")
	}
	if second.Header().Get("Content-Type") != "application/json" {
		t.Fatal("content type must be replayed")
	}
}

func TestOtherMethodsPassThroughWithoutKey(t *testing.T) {
	h := newHarness(t, created)
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/v1/collections", nil)
	rec := httptest.NewRecorder()
	h.mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || h.calls.Load() != 1 {
		t.Fatalf("GET must pass through, got %d", rec.Code)
	}
}

func TestRejectsMissingOrInvalidKey(t *testing.T) {
	cases := []struct{ name, key, code string }{
		{"missing", "", "idempotency_key_required"},
		{"space", "has space", "idempotency_key_invalid"},
		{"too long", strings.Repeat("a", 256), "idempotency_key_invalid"},
		{"non ascii", "clé", "idempotency_key_invalid"},
	}
	for _, c := range cases {
		h := newHarness(t, created)
		rec := h.post("{}", c.key)
		if rec.Code != http.StatusBadRequest || problemCode(t, rec) != c.code {
			t.Errorf("%s: got %d", c.name, rec.Code)
		}
		if h.calls.Load() != 0 {
			t.Errorf("%s: handler must not run", c.name)
		}
	}
}

func TestReusedKeyWithDifferentBodyIsRejected(t *testing.T) {
	h := newHarness(t, created)
	h.post(`{"a":1}`, "k1")
	rec := h.post(`{"a":2}`, "k1")
	code := problemCode(t, rec)
	if rec.Code != http.StatusUnprocessableEntity || code != "idempotency_key_reused" {
		t.Fatalf("got %d", rec.Code)
	}
	if h.calls.Load() != 1 {
		t.Fatal("handler must not run again")
	}
}

func TestHandlerReceivesFullBody(t *testing.T) {
	var got string
	h := newHarness(t, func(_ *harness, w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 64)
		n, _ := r.Body.Read(b)
		got = string(b[:n])
		w.WriteHeader(http.StatusOK)
	})
	h.post(`{"amount":100}`, "k1")
	if got != `{"amount":100}` {
		t.Fatalf("handler saw %q", got)
	}
}

func TestOversizedBodyIsRejected(t *testing.T) {
	h := newHarness(t, created)
	rec := h.post(strings.Repeat("x", 2048), "k1")
	if rec.Code != http.StatusRequestEntityTooLarge || h.calls.Load() != 0 {
		t.Fatalf("got %d, calls %d", rec.Code, h.calls.Load())
	}
}

func TestMissingScopeIsAServerError(t *testing.T) {
	h := newHarnessScope(t, func(*http.Request) (string, bool) { return "", false },
		created)
	rec := h.post("{}", "k1")
	if rec.Code != http.StatusInternalServerError || h.calls.Load() != 0 {
		t.Fatalf("got %d, calls %d", rec.Code, h.calls.Load())
	}
}
