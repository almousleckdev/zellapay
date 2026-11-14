package apiserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/webhooks"
	"github.com/zellapay/zellapay/internal/webhooks/webhookstest"
)

func endpointBody(url string, events ...string) string {
	b, _ := json.Marshal(map[string]any{"url": url, "events": events})
	return string(b)
}

// register creates an endpoint over the API and returns its ID and secret.
func (e *env) register(t *testing.T, url string, events ...string) (id, secret string) {
	t.Helper()
	idem := "idem-we-" + url[len(url)-5:]
	r := e.post("/v1/webhook-endpoints", e.main.key, idem, endpointBody(url, events...))
	if r.status != http.StatusCreated {
		t.Fatalf("register: %d %s", r.status, r.raw)
	}
	m := r.json(t)
	id, _ = m["id"].(string)
	secret, _ = m["secret"].(string)
	return id, secret
}

// pump runs the background work once: publish outbox events, then deliver.
func (e *env) pump(t *testing.T) webhooks.DeliverResult {
	t.Helper()
	if _, err := e.c.Webhooks.Dispatch(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	res, err := e.c.Webhooks.DeliverDue(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestAMerchantLearnsOfASuccessfulCollectionByWebhook(t *testing.T) {
	e := newEnv(t)
	recv := webhookstest.New()
	defer recv.Close()
	_, secret := e.register(t, recv.URL, webhooks.EventCollectionSucceeded)

	id, _ := e.create("00", "idem-wh-1").json(t)["id"].(string)
	e.clock.advance(3 * time.Second)
	e.deliverCallbacks()
	if res := e.pump(t); res.Succeeded != 1 {
		t.Fatalf("delivery: %+v", res)
	}

	got := recv.Last()
	if err := got.Verify(secret, e.clock.Now()); err != nil {
		t.Fatalf("signature: %v", err)
	}
	var event struct {
		ID, Type string
		Data     struct{ Object json.RawMessage }
	}
	if err := json.Unmarshal(got.Body, &event); err != nil || event.Type != "collection.succeeded" {
		t.Fatalf("payload %s: %v", got.Body, err)
	}
	// The embedded object is exactly what the API returns for the collection.
	api := e.get("/v1/collections/"+id, e.main.key)
	if !sameJSON(t, event.Data.Object, api.raw) {
		t.Fatalf("webhook object differs from the API:\n%s\n%s", event.Data.Object, api.raw)
	}
}

func sameJSON(t *testing.T, a, b []byte) bool {
	t.Helper()
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		t.Fatal("not JSON")
	}
	xa, _ := json.Marshal(x)
	ya, _ := json.Marshal(y)
	return string(xa) == string(ya)
}

func TestFailuresAreAnnouncedAndOnlyToSubscribers(t *testing.T) {
	e := newEnv(t)
	all, onlySuccess := webhookstest.New(), webhookstest.New()
	defer all.Close()
	defer onlySuccess.Close()
	e.register(t, all.URL, webhooks.EventCollectionSucceeded, webhooks.EventCollectionFailed)
	e.register(t, onlySuccess.URL, webhooks.EventCollectionSucceeded)

	e.create("01", "idem-wh-2") // the customer declines
	e.clock.advance(3 * time.Second)
	e.deliverCallbacks()
	e.pump(t)

	if all.Count() != 1 || onlySuccess.Count() != 0 {
		t.Fatalf("subscribed endpoint got %d, other got %d", all.Count(), onlySuccess.Count())
	}
	var event struct{ Type string }
	_ = json.Unmarshal(all.Last().Body, &event)
	if event.Type != "collection.failed" {
		t.Fatalf("type = %s", event.Type)
	}
}

func TestReplayThroughTheAPI(t *testing.T) {
	e := newEnv(t)
	recv := webhookstest.New()
	defer recv.Close()
	e.register(t, recv.URL, webhooks.EventCollectionSucceeded)
	e.create("00", "idem-wh-3")
	e.clock.advance(3 * time.Second)
	e.deliverCallbacks()
	e.pump(t)
	eventID := recv.Last().EventID()

	replay := "/v1/webhook-events/" + eventID + "/replay"
	if r := e.post(replay, e.main.key, "idem-replay-1", ""); r.status != http.StatusAccepted {
		t.Fatalf("replay: %d %s", r.status, r.raw)
	}
	e.pump(t)
	if recv.Count() != 2 {
		t.Fatalf("replay must deliver again, got %d requests", recv.Count())
	}
	other := e.post(replay, e.other.key, "idem-replay-2", "")
	if other.status != http.StatusForbidden && other.status != http.StatusNotFound {
		t.Fatalf("another merchant must not replay it: %d", other.status)
	}
}
