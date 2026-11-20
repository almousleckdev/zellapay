package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zellapay/zellapay/internal/platform/app"
	"github.com/zellapay/zellapay/internal/platform/config"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

func get(h http.Handler, path string) int {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), "GET", path, nil))
	return rec.Code
}

func gatewayHandler(t *testing.T, drain *httpx.Drain) http.Handler {
	t.Helper()
	cfg, err := config.Load("operator-gateway", ":8090")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return newHandler(log, cfg, drain)
}

func TestGatewayReadinessFollowsDrain(t *testing.T) {
	drain := httpx.NewDrain()
	h := gatewayHandler(t, drain)
	if get(h, "/healthz") != 200 || get(h, "/readyz") != 200 {
		t.Fatal("not healthy while serving")
	}
	drain.Start()
	if get(h, "/readyz") != 503 || get(h, "/healthz") != 200 {
		t.Fatal("readiness must fail and liveness must hold while draining")
	}
}

func TestGatewayRejectsOversizedBodies(t *testing.T) {
	h := gatewayHandler(t, nil)
	body := strings.NewReader(strings.Repeat("x", 2<<20))
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/anything", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestGatewayServerUsesSharedTimeouts(t *testing.T) {
	cfg, err := config.Load("operator-gateway", ":8090")
	if err != nil {
		t.Fatal(err)
	}
	srv := app.NewServer(cfg, gatewayHandler(t, nil))
	if srv.ReadHeaderTimeout != cfg.Server.ReadHeaderTimeout {
		t.Fatalf("read header timeout %v", srv.ReadHeaderTimeout)
	}
	if srv.WriteTimeout != cfg.Server.WriteTimeout {
		t.Fatalf("write timeout %v", srv.WriteTimeout)
	}
}
