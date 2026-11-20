package httpx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func limitedMux(t *testing.T, limits BodyLimits) (http.Handler, *int) {
	t.Helper()
	calls := new(int)
	read := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		if _, err := io.ReadAll(r.Body); err != nil {
			WriteError(w, r, Problem{Status: http.StatusBadRequest, Code: "bad", Message: "bad"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux := http.NewServeMux()
	mux.Handle("POST /callbacks/{operator}", read)
	mux.Handle("POST /v1/items", read)
	mux.Handle("POST /v1/bulk", read)
	return limits.Wrap(mux), calls
}

func post(h http.Handler, path string, body io.Reader, length int64) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), "POST", path, body)
	req.ContentLength = length
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestBodyLimitAllowsBodyAtTheLimit(t *testing.T) {
	h, _ := limitedMux(t, BodyLimits{Default: 8})
	if rec := post(h, "/v1/items", strings.NewReader("12345678"), 8); rec.Code != 204 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestBodyLimitRejectsDeclaredLengthBeforeTheHandler(t *testing.T) {
	h, calls := limitedMux(t, BodyLimits{Default: 8})
	rec := post(h, "/v1/items", strings.NewReader("123456789"), 9)
	if rec.Code != http.StatusRequestEntityTooLarge || *calls != 0 {
		t.Fatalf("status %d, handler calls %d", rec.Code, *calls)
	}
	if !strings.Contains(rec.Body.String(), `"code":"request_too_large"`) {
		t.Fatalf("not the standard envelope: %s", rec.Body.String())
	}
}

func TestBodyLimitCoversCallbacks(t *testing.T) {
	h, calls := limitedMux(t, BodyLimits{Default: 8})
	rec := post(h, "/callbacks/simulator", strings.NewReader("123456789"), 9)
	if rec.Code != http.StatusRequestEntityTooLarge || *calls != 0 {
		t.Fatalf("status %d, handler calls %d", rec.Code, *calls)
	}
}

func TestBodyLimitReturns413ForUndeclaredLengthWhenHandlerReportsAnError(t *testing.T) {
	h, _ := limitedMux(t, BodyLimits{Default: 8})
	rec := post(h, "/v1/items", strings.NewReader("123456789"), -1)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `"code":"bad"`) {
		t.Fatalf("handler body leaked through: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"request_too_large"`) {
		t.Fatalf("not the standard envelope: %s", rec.Body.String())
	}
}

func TestBodyLimitLeavesUndeclaredLengthWithinLimitAlone(t *testing.T) {
	h, _ := limitedMux(t, BodyLimits{Default: 8})
	if rec := post(h, "/v1/items", strings.NewReader("1234"), -1); rec.Code != 204 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestBodyLimitRouteOverrideRaisesTheLimitForThatRouteOnly(t *testing.T) {
	h, _ := limitedMux(t, BodyLimits{
		Default: 8,
		Routes:  map[string]int64{"POST /v1/bulk": 32},
	})
	body := strings.Repeat("x", 20)
	if rec := post(h, "/v1/bulk", strings.NewReader(body), 20); rec.Code != 204 {
		t.Fatalf("override route: status %d", rec.Code)
	}
	if rec := post(h, "/v1/items", strings.NewReader(body), 20); rec.Code != 413 {
		t.Fatalf("default route: status %d", rec.Code)
	}
	if rec := post(h, "/v1/bulk", strings.NewReader(strings.Repeat("x", 33)), 33); rec.Code != 413 {
		t.Fatalf("override route above its limit: status %d", rec.Code)
	}
}

func TestBodyLimitLeavesUnknownRoutesToTheMux(t *testing.T) {
	h, _ := limitedMux(t, BodyLimits{Default: 8})
	if rec := post(h, "/nope", strings.NewReader("1"), 1); rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestBodyLimitDoesNotRewriteSuccessfulResponses(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/lenient", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
	})
	h := BodyLimits{Default: 8}.Wrap(mux)
	if rec := post(h, "/v1/lenient", strings.NewReader("123456789"), -1); rec.Code != 202 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestDecodeJSONWorksUnderTheGlobalLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/items", func(w http.ResponseWriter, r *http.Request) {
		var v struct{ A string }
		if err := DecodeJSON(r, &v); err != nil {
			WriteError(w, r, Problem{Status: 400, Code: "bad_json", Message: "bad"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	h := BodyLimits{Default: 32}.Wrap(mux)
	if rec := post(h, "/v1/items", strings.NewReader(`{"A":"b"}`), 9); rec.Code != 204 {
		t.Fatalf("status %d", rec.Code)
	}
	long := `{"A":"` + strings.Repeat("b", 40) + `"}`
	if rec := post(h, "/v1/items", strings.NewReader(long), -1); rec.Code != 413 {
		t.Fatalf("oversized chunked JSON: status %d", rec.Code)
	}
}
