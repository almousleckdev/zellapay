package apiserver_test

import (
	"net/http"
	"testing"

	"github.com/zellapay/zellapay/internal/webhooks"
	"github.com/zellapay/zellapay/internal/webhooks/webhookstest"
)

func batchBody(items ...string) string {
	return `{"payouts":[` + joinComma(items) + `]}`
}

func joinComma(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

func TestBatchReportsEachItemSeparately(t *testing.T) {
	e := newEnv(t)
	e.fund(t, 7_000) // enough for two payouts of 3000, not three
	body := batchBody(payoutBody("00", 3000), payoutBody("00", 3000), payoutBody("00", 3000))
	r := e.post("/v1/payouts/batch", e.main.key, "idem-batch-1", body)
	if r.status != http.StatusMultiStatus {
		t.Fatalf("got %d %s", r.status, r.raw)
	}
	data, _ := r.json(t)["data"].([]any)
	if len(data) != 3 {
		t.Fatalf("data: %s", r.raw)
	}
	ok, refused := 0, 0
	for i, raw := range data {
		item := raw.(map[string]any)
		if int(item["index"].(float64)) != i {
			t.Errorf("item %d is out of order", i)
		}
		switch {
		case item["payout"] != nil:
			ok++
		case item["error"].(map[string]any)["code"] == "insufficient_funds":
			refused++
		}
	}
	if ok != 2 || refused != 1 {
		t.Fatalf("paid %d, refused %d; want 2 and 1", ok, refused)
	}
	if got := e.availableBalance(t); got != 1_000 {
		t.Fatalf("available = %d, want 1000", got)
	}
}

func TestRepeatingABatchNeverPaysAnyoneTwice(t *testing.T) {
	e := newEnv(t)
	e.fund(t, 20_000)
	body := batchBody(payoutBody("00", 3000), payoutBody("00", 4000))
	first := e.post("/v1/payouts/batch", e.main.key, "idem-batch-2", body)
	second := e.post("/v1/payouts/batch", e.main.key, "idem-batch-2", body)
	replayed := second.header.Get("Idempotent-Replayed") == "true"
	if string(first.raw) != string(second.raw) || !replayed {
		t.Fatal("the repeated batch must replay the original response")
	}
	if got := e.availableBalance(t); got != 13_000 {
		t.Fatalf("funds were held more than once: available = %d, want 13000", got)
	}
}

func TestBatchSizeIsBounded(t *testing.T) {
	e := newEnv(t)
	e.fund(t, 1_000)
	items := make([]string, 101)
	for i := range items {
		items[i] = payoutBody("00", 1)
	}
	for name, body := range map[string]string{
		"empty": `{"payouts":[]}`, "too many": batchBody(items...), "not json": `x`,
	} {
		r := e.post("/v1/payouts/batch", e.main.key, "idem-bb-"+name, body)
		if r.status != http.StatusBadRequest {
			t.Errorf("%s: got %d", name, r.status)
		}
	}
}

func TestAPayoutOutcomeIsDeliveredByWebhook(t *testing.T) {
	e := newEnv(t)
	e.fund(t, 20_000)
	recv := webhookstest.New()
	defer recv.Close()
	_, secret := e.register(t, recv.URL, webhooks.EventPayoutSucceeded, webhooks.EventPayoutFailed)

	id, _ := e.pay("00", "idem-pwh-1", 4_000).json(t)["id"].(string)
	e.clock.advance(3 * 1e9)
	e.deliverCallbacks()
	if res := e.pump(t); res.Succeeded != 1 {
		t.Fatalf("delivery: %+v", res)
	}
	got := recv.Last()
	if err := got.Verify(secret, e.clock.Now()); err != nil {
		t.Fatalf("signature: %v", err)
	}
	api := e.get("/v1/payouts/"+id, e.main.key)
	if !containsObject(t, got.Body, api.raw) {
		t.Fatalf("webhook object differs from the API:\n%s\n%s", got.Body, api.raw)
	}
}

// containsObject reports whether the webhook body's data.object equals obj.
func containsObject(t *testing.T, body, obj []byte) bool {
	t.Helper()
	var parsed struct {
		Type string `json:"type"`
		Data struct {
			Object jsonRaw `json:"object"`
		} `json:"data"`
	}
	if err := unmarshal(body, &parsed); err != nil || parsed.Type != "payout.succeeded" {
		t.Fatalf("payload %s: %v", body, err)
	}
	return sameJSON(t, parsed.Data.Object, obj)
}
