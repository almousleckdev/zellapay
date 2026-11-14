package httpauth

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

type stubAuth struct {
	p   merchants.Principal
	err error
}

func (s stubAuth) Authenticate(context.Context, string, netip.Addr) (merchants.Principal, error) {
	return s.p, s.err
}

func serve(
	t *testing.T, a Authenticator, scope merchants.Scope, header string,
) *httptest.ResponseRecorder {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := PrincipalFrom(r.Context())
		if !ok || p.MerchantID != "mer_1" {
			t.Error("principal missing from context")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	h := httpx.Middleware(log, Require(a, RemoteAddr, log)(RequireScope(scope)(final)))
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var b httpx.ErrorBody
	if err := json.NewDecoder(rec.Body).Decode(&b); err != nil {
		t.Fatal(err)
	}
	return b.Error.Code
}

var good = merchants.Principal{MerchantID: "mer_1", Scopes: []merchants.Scope{merchants.ScopeRead}}

func TestAllowsValidKeyWithScope(t *testing.T) {
	rec := serve(t, stubAuth{p: good}, merchants.ScopeRead, "Bearer zp_test_x")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestRejectsMissingOrMalformedHeader(t *testing.T) {
	for _, h := range []string{"", "Basic abc", "Bearer", "Bearer  "} {
		rec := serve(t, stubAuth{p: good}, merchants.ScopeRead, h)
		if rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "invalid_api_key" {
			t.Errorf("%q: got %d", h, rec.Code)
		}
		if rec.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("%q: missing WWW-Authenticate", h)
		}
	}
}

func TestMapsAuthenticationErrors(t *testing.T) {
	cases := []struct {
		err  error
		code int
		name string
	}{
		{merchants.ErrInvalidKey, 401, "invalid_api_key"},
		{merchants.ErrWrongMode, 401, "invalid_api_key"},
		{merchants.ErrMerchantInactive, 403, "merchant_inactive"},
		{merchants.ErrIPNotAllowed, 403, "ip_not_allowed"},
		{io.ErrUnexpectedEOF, 500, "internal_error"},
	}
	for _, c := range cases {
		rec := serve(t, stubAuth{err: c.err}, merchants.ScopeRead, "Bearer zp_test_x")
		if rec.Code != c.code || errorCode(t, rec) != c.name {
			t.Errorf("%v: got %d", c.err, rec.Code)
		}
	}
}

func TestRejectsMissingScope(t *testing.T) {
	rec := serve(t, stubAuth{p: good}, merchants.ScopePayoutsWrite, "Bearer zp_test_x")
	if rec.Code != http.StatusForbidden || errorCode(t, rec) != "insufficient_scope" {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestErrorsNeverLeakKeyMaterial(t *testing.T) {
	a := stubAuth{err: merchants.ErrInvalidKey}
	rec := serve(t, a, merchants.ScopeRead, "Bearer zp_test_SECRETVALUE")
	if body := rec.Body.String(); len(body) == 0 || strings.Contains(body, "SECRETVALUE") {
		t.Fatalf("response leaked key material: %s", body)
	}
}
