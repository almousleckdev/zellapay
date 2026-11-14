package apiserver_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHealthEndpoints(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/healthz", "/readyz"} {
		if r := e.get(p, ""); r.status != http.StatusOK {
			t.Errorf("%s: %d", p, r.status)
		}
	}
}

func TestRequestsWithoutAValidKeyAreRefused(t *testing.T) {
	e := newEnv(t)
	wrongSecret := "zp_test_0000000000000000_" + strings.Repeat("A", 43)
	for name, key := range map[string]string{"none": "", "garbage": "x", "wrong": wrongSecret} {
		for _, r := range []response{
			e.get("/v1/collections", key),
			e.post("/v1/collections", key, "k", "{}"),
		} {
			if r.status != http.StatusUnauthorized || r.errorCode(t) != "invalid_api_key" {
				t.Errorf("%s: got %d %s", name, r.status, r.raw)
			}
		}
	}
}

func TestAReadOnlyKeyCannotCreate(t *testing.T) {
	e := newEnv(t)
	r := e.post("/v1/collections", e.reader.key, "idem-scope-1", body("00", "ORDER-00", 10000))
	if r.status != http.StatusForbidden || r.errorCode(t) != "insufficient_scope" {
		t.Fatalf("got %d %s", r.status, r.raw)
	}
	if ok := e.get("/v1/collections", e.reader.key); ok.status != http.StatusOK {
		t.Fatalf("a read key may read, got %d", ok.status)
	}
}

func TestCreateRequiresAnIdempotencyKey(t *testing.T) {
	e := newEnv(t)
	r := e.post("/v1/collections", e.main.key, "", body("00", "O", 10000))
	if r.status != http.StatusBadRequest || r.errorCode(t) != "idempotency_key_required" {
		t.Fatalf("got %d %s", r.status, r.raw)
	}
}

func TestRevokedKeyStopsWorkingImmediately(t *testing.T) {
	e := newEnv(t)
	keys, err := e.c.Merchants.ListKeys(contextBackground(), e.other.merchant.ID)
	if err != nil || len(keys) == 0 {
		t.Fatalf("keys: %v, %v", keys, err)
	}
	if r := e.get("/v1/collections", e.other.key); r.status != http.StatusOK {
		t.Fatalf("before revoke: %d", r.status)
	}
	err = e.c.Merchants.RevokeKey(contextBackground(), e.other.merchant.ID, keys[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if r := e.get("/v1/collections", e.other.key); r.status != http.StatusUnauthorized {
		t.Fatalf("after revoke: %d", r.status)
	}
}

func TestCallbackEndpointRefusesForgeriesAndStrangers(t *testing.T) {
	e := newEnv(t)
	e.create("00", "idem-cb-1")
	e.clock.advance(3 * time.Second)
	forged := e.do(request{
		method: "POST", path: "/callbacks/simulator",
		body: `{"reference":"x","status":"SUCCESS"}`,
	})
	if forged.status != http.StatusBadRequest {
		t.Fatalf("an unsigned callback must be refused, got %d", forged.status)
	}
	stranger := e.post("/callbacks/nobody", "", "", "{}")
	if stranger.status != http.StatusNotFound {
		t.Fatalf("unknown operator: %d", stranger.status)
	}
	if n := e.deliverCallbacks(); n != 1 {
		t.Fatalf("the genuine callback must still work, got %d", n)
	}
}
