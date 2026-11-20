package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const secretPanic = "boom-secret-detail"

func serve(h http.Handler) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), "GET", "/pay", nil))
	return rec
}

func TestRecoverBeforeCommitReturnsSanitizedEnvelope(t *testing.T) {
	log, _ := newLogSink()
	h := Middleware(log, Recover(log)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(secretPanic)
	})))
	rec := serve(h)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), secretPanic) {
		t.Fatalf("panic detail leaked: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("not the standard envelope: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), rec.Header().Get("X-Request-ID")) {
		t.Fatalf("request id missing from envelope: %s", rec.Body.String())
	}
}

func TestRecoverLogsRequestIDAndStack(t *testing.T) {
	log, sink := newLogSink()
	h := Middleware(log, Recover(log)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(secretPanic)
	})))
	rec := serve(h)
	got := sink.find(t, "panic recovered")
	if got["request_id"] != rec.Header().Get("X-Request-ID") {
		t.Fatalf("request id: %v", got["request_id"])
	}
	if got["panic"] != secretPanic {
		t.Fatalf("panic value: %v", got["panic"])
	}
	if !strings.Contains(got["stack"].(string), "recover_test.go") {
		t.Fatalf("stack does not point at the panic site: %v", got["stack"])
	}
	if sink.find(t, "request")["status"] != float64(500) {
		t.Fatalf("access log did not record 500")
	}
}

func TestRecoverAfterCommitWritesNothingMore(t *testing.T) {
	log, sink := newLogSink()
	commitThenPanic := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("partial"))
		panic(secretPanic)
	})
	h := Middleware(log, Recover(log)(commitThenPanic))
	rec := serve(h)
	if rec.Code != http.StatusAccepted || rec.Body.String() != "partial" {
		t.Fatalf("response was altered: %d %q", rec.Code, rec.Body.String())
	}
	sink.find(t, "panic recovered")
}

func TestRecoverAfterBodyWriteWithoutHeaderWritesNothingMore(t *testing.T) {
	log, _ := newLogSink()
	h := Recover(log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("partial"))
		panic(secretPanic)
	}))
	rec := serve(h)
	if rec.Body.String() != "partial" {
		t.Fatalf("response was altered: %q", rec.Body.String())
	}
}

func TestRecoverWorksWithoutOuterMiddleware(t *testing.T) {
	log, _ := newLogSink()
	rec := serve(Recover(log)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(secretPanic)
	})))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestRecoverRepanicsAbortHandler(t *testing.T) {
	log, _ := newLogSink()
	h := Recover(log)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	defer func() {
		err, _ := recover().(error)
		if !errors.Is(err, http.ErrAbortHandler) {
			t.Fatal("ErrAbortHandler must propagate")
		}
	}()
	serve(h)
}

func TestRecoverPassesThroughNormalResponses(t *testing.T) {
	log, _ := newLogSink()
	rec := serve(Recover(log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d", rec.Code)
	}
}
