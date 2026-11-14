package apiserver_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/zellapay/zellapay/internal/webhooks/webhookstest"
)

const (
	okURL      = "https://example.com/h"
	badScheme  = `{"url":"ftp://x.example/h","events":["collection.succeeded"]}`
	extraField = `{"url":"` + okURL + `","events":["collection.failed"],"x":1}`
)

func TestEndpointSecretIsShownOnlyAtCreation(t *testing.T) {
	e := newEnv(t)
	recv := webhookstest.New()
	defer recv.Close()
	id, secret := e.register(t, recv.URL, "collection.succeeded")
	if !strings.HasPrefix(secret, "whsec_") || id == "" {
		t.Fatalf("id=%q secret=%q", id, secret)
	}
	list := e.get("/v1/webhook-endpoints", e.main.key)
	body := string(list.raw)
	if strings.Contains(body, secret) || strings.Contains(body, `"secret"`) {
		t.Fatalf("the list must never include a secret: %s", list.raw)
	}
	if data, _ := list.json(t)["data"].([]any); len(data) != 1 {
		t.Fatalf("list: %s", list.raw)
	}
}

func TestEndpointsAreVisibleOnlyToTheirMerchant(t *testing.T) {
	e := newEnv(t)
	recv := webhookstest.New()
	defer recv.Close()
	e.register(t, recv.URL, "collection.succeeded")
	list := e.get("/v1/webhook-endpoints", e.other.key).json(t)
	if data, _ := list["data"].([]any); len(data) != 0 {
		t.Fatalf("another merchant's endpoints leaked: %v", list)
	}
}

func TestWebhookManagementNeedsItsOwnScope(t *testing.T) {
	e := newEnv(t)
	r := e.post("/v1/webhook-endpoints", e.reader.key, "idem-we-scope",
		endpointBody("https://example.com/h", "collection.succeeded"))
	if r.status != http.StatusForbidden || r.errorCode(t) != "insufficient_scope" {
		t.Fatalf("got %d %s", r.status, r.raw)
	}
	if ok := e.get("/v1/webhook-endpoints", e.reader.key); ok.status != http.StatusOK {
		t.Fatalf("a read key may list endpoints: %d", ok.status)
	}
}

func TestBadEndpointRequestsAreRefused(t *testing.T) {
	e := newEnv(t)
	cases := map[string]struct {
		body string
		code string
		http int
	}{
		"bad scheme":    {badScheme, "unsafe_url", 422},
		"unknown event": {endpointBody(okURL, "nope"), "invalid_request", 400},
		"no events":     {`{"url":"` + okURL + `","events":[]}`, "invalid_request", 400},
		"unknown field": {extraField, "invalid_request", 400},
		"not json":      {`nope`, "invalid_request", 400},
	}
	n := 0
	for name, c := range cases {
		n++
		r := e.post("/v1/webhook-endpoints", e.main.key, "idem-bad-"+string(rune('a'+n)), c.body)
		if r.status != c.http || r.errorCode(t) != c.code {
			t.Errorf("%s: got %d %s", name, r.status, r.raw)
		}
	}
}

func TestReplayOfAnUnknownEventIsNotFound(t *testing.T) {
	e := newEnv(t)
	r := e.post("/v1/webhook-events/evt_nope/replay", e.main.key, "idem-nf-1", "")
	if r.status != http.StatusNotFound || r.errorCode(t) != "not_found" {
		t.Fatalf("got %d %s", r.status, r.raw)
	}
}

func TestWebhookEndpointsRequireAuthentication(t *testing.T) {
	e := newEnv(t)
	for _, r := range []response{
		e.get("/v1/webhook-endpoints", ""),
		e.post("/v1/webhook-endpoints", "", "k", "{}"),
		e.post("/v1/webhook-events/evt_1/replay", "", "k", ""),
	} {
		if r.status != http.StatusUnauthorized {
			t.Errorf("got %d", r.status)
		}
	}
}
