package httpx

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareSetsRequestID(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := Middleware(log, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if RequestID(r.Context()) == "" {
			t.Error("request id missing from context")
		}
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), "GET", "/", nil))
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID header not set")
	}
}

func TestHealth(t *testing.T) {
	mux := http.NewServeMux()
	Health(mux, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), "GET", "/healthz", nil))
	if rec.Code != 200 {
		t.Fatalf("got %d", rec.Code)
	}
}
