package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type sample struct {
	Amount int64  `json:"amount"`
	Name   string `json:"name"`
}

func decode(t *testing.T, body string) (sample, error) {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost, "/", strings.NewReader(body))
	var s sample
	return s, DecodeJSON(req, &s)
}

func TestDecodeJSONAcceptsExactlyOneValidValue(t *testing.T) {
	s, err := decode(t, `{"amount": 10000, "name": "x"}`)
	if err != nil || s.Amount != 10000 || s.Name != "x" {
		t.Fatalf("got %+v, %v", s, err)
	}
}

func TestDecodeJSONRejectsBadBodies(t *testing.T) {
	bad := map[string]string{
		"empty":         ``,
		"not json":      `nope`,
		"unknown field": `{"amount":1,"extra":true}`,
		"trailing data": `{"amount":1} {"amount":2}`,
		"wrong type":    `{"amount":"ten"}`,
		"fractional":    `{"amount":10.5}`,
		"array":         `[1]`,
	}
	for name, body := range bad {
		if _, err := decode(t, body); !errors.Is(err, ErrBadJSON) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestChainRunsFirstMiddlewareFirst(t *testing.T) {
	var order []string
	mw := func(name string) Decorator {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		order = append(order, "handler")
	}), mw("a"), mw("b"))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/", nil))
	if strings.Join(order, ",") != "a,b,handler" {
		t.Fatalf("order = %v", order)
	}
}
