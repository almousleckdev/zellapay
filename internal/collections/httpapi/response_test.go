package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/zellapay/zellapay/internal/collections"
)

func body(t *testing.T, c collections.Collection) map[string]any {
	t.Helper()
	h := newHandler(&fakeService{collection: c}, c.MerchantID)
	rec := get(h.Get, "/v1/collections/x")
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestInternalStatesAreHiddenFromMerchants(t *testing.T) {
	for _, s := range []collections.Status{
		collections.StatusCreated, collections.StatusPending, collections.StatusAmbiguous,
	} {
		c := sampleCollection()
		c.Status = s
		if got := body(t, c)["status"]; got != "pending" {
			t.Errorf("%s shows as %v, want pending", s, got)
		}
	}
}

func TestFeeAppearsOnlyOnceSucceeded(t *testing.T) {
	c := sampleCollection()
	if _, present := body(t, c)["fee"]; present {
		t.Fatal("fee must not appear while pending")
	}
	c.Status = collections.StatusSucceeded
	if got := body(t, c)["fee"]; got != float64(150) {
		t.Fatalf("fee = %v", got)
	}
}

func TestFailureCodesArePublicAndStable(t *testing.T) {
	cases := map[string]string{
		"CUSTOMER_DECLINED":       "customer_declined",
		"INSUFFICIENT_FUNDS":      "insufficient_customer_funds",
		"rejected":                "rejected",
		"operator_never_received": "operator_unavailable",
		"SOME_NEW_OPERATOR_CODE":  "operator_error",
		"":                        "operator_error",
	}
	for internal, want := range cases {
		c := sampleCollection()
		c.Status, c.FailureCode = collections.StatusFailed, internal
		if got := body(t, c)["failure_code"]; got != want {
			t.Errorf("%q -> %v, want %s", internal, got, want)
		}
	}
}

func TestHeldCollectionLeaksNoInternalDetail(t *testing.T) {
	c := sampleCollection()
	c.Status, c.FailureCode = collections.StatusAmbiguous, collections.CodeAmountMismatch
	m := body(t, c)
	if m["failure_code"] != nil || m["status"] != "pending" {
		t.Fatalf("a held collection must look pending with no failure: %v", m)
	}
}

func TestGetHidesOtherMerchantsCollections(t *testing.T) {
	f := &fakeService{collection: sampleCollection()}
	rec := get(newHandler(f, "mer_other").Get, "/v1/collections/col_1")
	if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.String()) != "not_found" {
		t.Fatalf("got %d", rec.Code)
	}
	missing := newHandler(&fakeService{err: collections.ErrNotFound}, "mer_1")
	rec = get(missing.Get, "/")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing collection got %d", rec.Code)
	}
}
