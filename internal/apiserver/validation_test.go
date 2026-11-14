package apiserver_test

import (
	"fmt"
	"net/http"
	"testing"
)

func payload(phone, currency string) string {
	return `{"amount":100,"currency":"` + currency + `","customer_phone":"` + phone +
		`","merchant_reference":"O"}`
}

func TestInvalidRequestsAreRejectedWithoutSideEffects(t *testing.T) {
	e := newEnv(t)
	cases := map[string]struct {
		body string
		code string
		http int
	}{
		"not json":        {`nope`, "invalid_request", 400},
		"unknown field":   {`{"amount":1,"currency":"XAF","metadata":{}}`, "invalid_request", 400},
		"zero amount":     {body("00", "O", 0), "invalid_request", 400},
		"bad phone":       {payload("0707", "XAF"), "invalid_request", 400},
		"lowercase ccy":   {payload("+24107000000", "xaf"), "invalid_request", 400},
		"unserved number": {payload("+33612345678", "XAF"), "unsupported_number", 422},
	}
	n := 0
	for name, c := range cases {
		n++
		r := e.post("/v1/collections", e.main.key, fmt.Sprintf("idem-val-%d", n), c.body)
		if r.status != c.http || r.errorCode(t) != c.code {
			t.Errorf("%s: got %d %s", name, r.status, r.raw)
		}
	}
	list := e.get("/v1/collections", e.main.key).json(t)
	if data, _ := list["data"].([]any); len(data) != 0 {
		t.Fatalf("nothing may be created by invalid requests: %v", list)
	}
}

func TestRejectedByTheOperatorFailsImmediately(t *testing.T) {
	e := newEnv(t)
	r := e.create("08", "idem-rej-1")
	got := r.json(t)
	if r.status != http.StatusCreated || got["status"] != "failed" {
		t.Fatalf("got %d %s", r.status, r.raw)
	}
	if got["failure_code"] != "rejected" {
		t.Fatalf("failure_code = %v", got["failure_code"])
	}
}
