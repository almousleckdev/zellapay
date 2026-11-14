package apiserver_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/ledger"
	ledgerstore "github.com/zellapay/zellapay/internal/ledger/pgstore"
)

func body(phoneSuffix, ref string, amount int) string {
	return `{"amount":` + itoa(amount) + `,"currency":"XAF","customer_phone":"+241070000` +
		phoneSuffix + `","merchant_reference":"` + ref + `"}`
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for ; n > 0; n /= 10 {
		d = append([]byte{byte('0' + n%10)}, d...)
	}
	return string(d)
}

func (e *env) create(suffix, idem string) response {
	return e.post("/v1/collections", e.main.key, idem, body(suffix, "ORDER-"+suffix, 10000))
}

func TestCollectionLifecycleOverHTTP(t *testing.T) {
	e := newEnv(t)
	created := e.create("00", "idem-life-1")
	id := e.assertCreatedPending(t, created)

	e.clock.advance(3 * time.Second)
	if n := e.deliverCallbacks(); n != 1 {
		t.Fatalf("want 1 callback, got %d", n)
	}
	got := e.get("/v1/collections/"+id, e.main.key).json(t)
	if got["status"] != "succeeded" || got["fee"] != float64(150) || got["completed_at"] == nil {
		t.Fatalf("after the callback: %v", got)
	}
	e.assertMerchantPending(t, 9850)
}

// assertCreatedPending checks a fresh creation and returns the new ID.
func (e *env) assertCreatedPending(t *testing.T, r response) string {
	t.Helper()
	got := r.json(t)
	id, _ := got["id"].(string)
	if r.status != http.StatusCreated || got["status"] != "pending" {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	if r.header.Get("Location") != "/v1/collections/"+id {
		t.Fatalf("Location = %q", r.header.Get("Location"))
	}
	if strings.Contains(string(r.raw), "+24107000000") {
		t.Fatal("the full phone number must never be returned")
	}
	return id
}

func (e *env) assertMerchantPending(t *testing.T, want int64) {
	t.Helper()
	store := ledgerstore.New(e.c.Pool)
	got, err := store.Balance(context.Background(), ledger.MerchantPending(e.main.merchant.ID))
	if err != nil || got != want {
		t.Fatalf("merchant pending = %d, %v; want %d", got, err, want)
	}
}

func TestRetryReplaysTheStoredResponse(t *testing.T) {
	e := newEnv(t)
	first := e.create("00", "idem-retry-1")
	second := e.create("00", "idem-retry-1")
	if second.status != http.StatusCreated || string(second.raw) != string(first.raw) {
		t.Fatalf("replay differs: %d %s", second.status, second.raw)
	}
	if second.header.Get("Idempotent-Replayed") != "true" {
		t.Fatal("a replay must say so")
	}
	list := e.get("/v1/collections", e.main.key).json(t)
	if data, _ := list["data"].([]any); len(data) != 1 {
		t.Fatalf("a retry must not create a second collection: %v", list)
	}
}

func TestReusingAKeyForADifferentRequestIsRefused(t *testing.T) {
	e := newEnv(t)
	e.create("00", "idem-reuse-1")
	r := e.post("/v1/collections", e.main.key, "idem-reuse-1", body("00", "ORDER-00", 20000))
	if r.status != http.StatusUnprocessableEntity || r.errorCode(t) != "idempotency_key_reused" {
		t.Fatalf("got %d %s", r.status, r.raw)
	}
}

func TestMerchantsCannotSeeEachOthersCollections(t *testing.T) {
	e := newEnv(t)
	id, _ := e.create("00", "idem-own-1").json(t)["id"].(string)
	r := e.get("/v1/collections/"+id, e.other.key)
	if r.status != http.StatusNotFound || r.errorCode(t) != "not_found" {
		t.Fatalf("got %d %s", r.status, r.raw)
	}
	list := e.get("/v1/collections", e.other.key).json(t)
	if data, _ := list["data"].([]any); len(data) != 0 {
		t.Fatalf("another merchant's list must be empty: %v", list)
	}
}

func TestDeclinedCollectionReportsAPublicFailureCode(t *testing.T) {
	e := newEnv(t)
	id, _ := e.create("01", "idem-decl-1").json(t)["id"].(string)
	e.clock.advance(3 * time.Second)
	e.deliverCallbacks()
	got := e.get("/v1/collections/"+id, e.main.key).json(t)
	if got["status"] != "failed" || got["failure_code"] != "customer_declined" {
		t.Fatalf("got %v", got)
	}
}

func TestTimeoutLooksPendingThenResolves(t *testing.T) {
	e := newEnv(t)
	created := e.create("03", "idem-time-1")
	if created.status != http.StatusCreated || created.json(t)["status"] != "pending" {
		t.Fatalf("an unclear outcome must look pending: %s", created.raw)
	}
	id, _ := created.json(t)["id"].(string)
	e.clock.advance(15 * time.Second)
	if _, err := e.c.Collections.Sweep(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	got := e.get("/v1/collections/"+id, e.main.key).json(t)
	if got["status"] != "succeeded" {
		t.Fatalf("got %v", got)
	}
}
