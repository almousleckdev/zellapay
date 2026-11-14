package httpsender

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/webhooks"
)

var dev = webhooks.Policy{AllowPrivate: true, AllowHTTP: true}

func send(t *testing.T, s *Sender, url, body string) (int, error) {
	t.Helper()
	return s.Send(context.Background(), webhooks.SendRequest{
		URL: url, Body: []byte(body), Headers: map[string]string{"X-Test": "1"},
	})
}

func code(t *testing.T, err error) string {
	t.Helper()
	var se *webhooks.SendError
	if !errors.As(err, &se) {
		t.Fatalf("want a SendError, got %v", err)
	}
	return se.Code
}

func TestSendDeliversBodyAndHeaders(t *testing.T) {
	var gotBody, gotHeader, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody, gotHeader, gotMethod = string(b), r.Header.Get("X-Test"), r.Method
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	status, err := send(t, New(dev, time.Second), srv.URL, `{"a":1}`)
	if err != nil || status != http.StatusAccepted {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if gotBody != `{"a":1}` || gotHeader != "1" || gotMethod != http.MethodPost {
		t.Fatalf("server saw %q %q %q", gotMethod, gotBody, gotHeader)
	}
}

func TestProductionPolicyRefusesToConnectToLoopback(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(counting(&hits))
	defer srv.Close()
	_, err := send(t, New(webhooks.Policy{AllowHTTP: true}, time.Second), srv.URL, "{}")
	if code(t, err) != webhooks.CodeBlockedHost || hits.Load() != 0 {
		t.Fatalf("a blocked address must never be contacted: err=%v hits=%d", err, hits.Load())
	}
}

func TestRedirectsAreNeverFollowed(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(counting(&targetHits))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirector.Close()
	status, err := send(t, New(dev, time.Second), redirector.URL, "{}")
	if err != nil || status != http.StatusFound || targetHits.Load() != 0 {
		t.Fatalf("status=%d err=%v targetHits=%d", status, err, targetHits.Load())
	}
}

func TestSlowEndpointsTimeOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	start := time.Now()
	_, err := send(t, New(dev, 150*time.Millisecond), srv.URL, "{}")
	if code(t, err) != webhooks.CodeTimeout || time.Since(start) > time.Second {
		t.Fatalf("err=%v after %v", err, time.Since(start))
	}
}

func TestConnectionAndCertificateFailuresAreClassified(t *testing.T) {
	closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := closed.URL
	closed.Close()
	_, err := send(t, New(dev, time.Second), url, "{}")
	if code(t, err) != webhooks.CodeConnection {
		t.Errorf("closed port: %v", err)
	}
	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer tlsSrv.Close()
	_, err = send(t, New(dev, time.Second), tlsSrv.URL, "{}")
	if code(t, err) != webhooks.CodeTLS {
		t.Errorf("self-signed certificate: %v", err)
	}
}

func TestErrorsNeverRevealInternalAddresses(t *testing.T) {
	strict := New(webhooks.Policy{AllowHTTP: true}, time.Second)
	_, err := send(t, strict, "http://127.0.0.1:1/x", "{}")
	if strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("the error leaks the address: %v", err)
	}
}

func TestHugeResponsesDoNotStallTheSender(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 5<<20)))
	}))
	defer srv.Close()
	status, err := send(t, New(dev, 2*time.Second), srv.URL, "{}")
	if err != nil || status != http.StatusOK {
		t.Fatalf("status=%d err=%v", status, err)
	}
}

// counting returns a handler that counts the requests it receives.
func counting(n *atomic.Int32) http.Handler {
	return http.HandlerFunc(func(http.ResponseWriter, *http.Request) { n.Add(1) })
}
