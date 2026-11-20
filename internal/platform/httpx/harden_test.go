package httpx

import (
	"net/http"
	"strings"
	"testing"
)

func hardened(t *testing.T, h http.HandlerFunc) http.Handler {
	t.Helper()
	log, _ := newLogSink()
	mux := http.NewServeMux()
	mux.Handle("POST /v1/items", h)
	mux.Handle("POST /callbacks/{operator}", h)
	Health(mux, nil)
	return Harden(log, mux, BodyLimits{Default: 8})
}

func TestHardenRecoversPanicsOnEveryRoute(t *testing.T) {
	h := hardened(t, func(http.ResponseWriter, *http.Request) { panic(secretPanic) })
	for _, path := range []string{"/v1/items", "/callbacks/simulator"} {
		rec := post(h, path, strings.NewReader("1"), 1)
		leaked := strings.Contains(rec.Body.String(), secretPanic)
		if rec.Code != http.StatusInternalServerError || leaked {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestHardenLimitsBodiesOnEveryRoute(t *testing.T) {
	h := hardened(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
	for _, path := range []string{"/v1/items", "/callbacks/simulator"} {
		rec := post(h, path, strings.NewReader("123456789"), 9)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("%s: %d", path, rec.Code)
		}
		if rec.Header().Get("X-Request-ID") == "" {
			t.Fatalf("%s: no request id on the 413", path)
		}
	}
}

func TestHardenKeepsHealthEndpoints(t *testing.T) {
	h := hardened(t, func(http.ResponseWriter, *http.Request) {})
	if healthz(h) != 200 || readyz(h) != 200 {
		t.Fatal("health endpoints broken")
	}
}
