package apiserver_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zellapay/zellapay/internal/apiserver"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

func postRaw(t *testing.T, url string, body []byte) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

func TestOversizedBodiesAreRejectedOnEveryPublicRoute(t *testing.T) {
	e := newEnv(t)
	big := bytes.Repeat([]byte("x"), 2<<20)
	for _, path := range []string{"/callbacks/simulator", "/v1/collections", "/v1/payouts"} {
		status, raw := postRaw(t, e.srv.URL+path, big)
		if status != http.StatusRequestEntityTooLarge {
			t.Errorf("%s: %d %s", path, status, raw)
		}
		if !strings.Contains(raw, `"code":"request_too_large"`) {
			t.Errorf("%s: not the standard envelope: %s", path, raw)
		}
	}
}

func TestReadinessFailsOnceDraining(t *testing.T) {
	e := newEnv(t)
	drain := httpx.NewDrain()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(apiserver.New(e.c, apiserver.Options{Drain: drain}, log))
	t.Cleanup(srv.Close)
	status := func(path string) int {
		req, err := http.NewRequestWithContext(context.Background(), "GET", srv.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	if status("/readyz") != 200 {
		t.Fatal("not ready while serving")
	}
	drain.Start()
	if status("/readyz") != 503 || status("/healthz") != 200 {
		t.Fatal("readiness must fail and liveness must hold while draining")
	}
}
