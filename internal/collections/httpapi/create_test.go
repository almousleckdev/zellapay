package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

const validBody = `{"amount":10000,"currency":"XAF","customer_phone":"+24107000000",` +
	`"merchant_reference":"ORDER-1","description":"Order 1"}`

func errorCode(t *testing.T, body string) string {
	t.Helper()
	var b httpx.ErrorBody
	if err := json.Unmarshal([]byte(body), &b); err != nil {
		t.Fatalf("not an error envelope: %s", body)
	}
	return b.Error.Code
}

func TestCreatePassesTheRequestToTheService(t *testing.T) {
	f := &fakeService{collection: sampleCollection()}
	rec := do(newHandler(f, "mer_1").Create, call{
		method: "POST", target: "/v1/collections", body: validBody,
		header: map[string]string{"Idempotency-Key": "key-1"},
	})

	if rec.Code != http.StatusCreated || rec.Header().Get("Location") != "/v1/collections/col_1" {
		t.Fatalf("got %d, %q", rec.Code, rec.Header().Get("Location"))
	}
	want := collections.CreateInput{
		MerchantID: "mer_1", IdempotencyKey: "key-1", Reference: "ORDER-1",
		Phone: "+24107000000", Currency: "XAF", Description: "Order 1", Amount: 10000,
	}
	if f.created != want {
		t.Fatalf("service got %+v", f.created)
	}
}

func TestCreateNeverReturnsTheFullPhoneNumber(t *testing.T) {
	f := &fakeService{collection: sampleCollection()}
	rec := post(newHandler(f, "mer_1").Create, "/v1/collections", validBody)
	body := rec.Body.String()
	if strings.Contains(body, "+24107000000") || !strings.Contains(body, "+241 07 ** ** 56") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestCreateRejectsBadBodies(t *testing.T) {
	bodies := map[string]string{
		"not json":      `nope`,
		"unknown field": `{"amount":1,"currency":"XAF","metadata":{}}`,
		"wrong type":    `{"amount":"ten"}`,
		"two objects":   validBody + validBody,
	}
	for name, body := range bodies {
		f := &fakeService{}
		rec := post(newHandler(f, "mer_1").Create, "/v1/collections", body)
		code := errorCode(t, rec.Body.String())
		if rec.Code != http.StatusBadRequest || code != "invalid_request" {
			t.Errorf("%s: got %d", name, rec.Code)
		}
		if f.created.MerchantID != "" {
			t.Errorf("%s: the service must not be called", name)
		}
	}
}

func TestCreateMapsServiceErrors(t *testing.T) {
	cases := []struct {
		err  error
		code int
		name string
	}{
		{fmt.Errorf("%w: phone", collections.ErrInvalidInput), 400, "invalid_request"},
		{collections.ErrUnsupportedNumber, 422, "unsupported_number"},
		{collections.ErrIdempotencyConflict, 422, "idempotency_key_reused"},
		{errors.New("pq: connection refused to 10.0.0.5"), 500, "internal_error"},
	}
	for _, c := range cases {
		h := newHandler(&fakeService{err: c.err}, "mer_1")
		rec := post(h.Create, "/v1/collections", validBody)
		if rec.Code != c.code || errorCode(t, rec.Body.String()) != c.name {
			t.Errorf("%v: got %d %s", c.err, rec.Code, rec.Body)
		}
	}
}

func TestValidationMessageIsShownButInternalsAreNot(t *testing.T) {
	bad := fmt.Errorf("%w: phone must be E.164", collections.ErrInvalidInput)
	rec := post(newHandler(&fakeService{err: bad}, "mer_1").Create, "/", validBody)
	if !strings.Contains(rec.Body.String(), "phone must be E.164") {
		t.Fatalf("validation detail missing: %s", rec.Body)
	}
	internal := errors.New("pq: connection refused to 10.0.0.5")
	rec = post(newHandler(&fakeService{err: internal}, "mer_1").Create, "/", validBody)
	out := rec.Body.String()
	if strings.Contains(out, "10.0.0.5") || strings.Contains(out, "pq:") {
		t.Fatalf("internal detail leaked: %s", rec.Body)
	}
}

func TestMissingMerchantIsAServerError(t *testing.T) {
	rec := post(newHandler(&fakeService{}, "").Create, "/", validBody)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d", rec.Code)
	}
}
