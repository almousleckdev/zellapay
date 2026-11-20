package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func readyz(h http.Handler) int {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), "GET", "/readyz", nil))
	return rec.Code
}

func healthz(h http.Handler) int {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), "GET", "/healthz", nil))
	return rec.Code
}

func healthMux(ready func(context.Context) error) http.Handler {
	mux := http.NewServeMux()
	Health(mux, ready)
	return mux
}

func TestReadyWhileServingAndNotReadyWhileDraining(t *testing.T) {
	d := NewDrain()
	h := healthMux(d.Check(time.Second, func(context.Context) error { return nil }))
	if got := readyz(h); got != 200 {
		t.Fatalf("serving: %d", got)
	}
	d.Start()
	if got := readyz(h); got != 503 {
		t.Fatalf("draining: %d", got)
	}
}

func TestLivenessIgnoresDraining(t *testing.T) {
	d := NewDrain()
	d.Start()
	h := healthMux(d.Check(time.Second, nil))
	if got := healthz(h); got != 200 {
		t.Fatalf("healthz: %d", got)
	}
}

func TestDrainStartIsIdempotentAndSignalsOnce(t *testing.T) {
	d := NewDrain()
	d.Start()
	d.Start()
	select {
	case <-d.Started():
	default:
		t.Fatal("Started channel not closed")
	}
	if !d.Draining() {
		t.Fatal("not draining")
	}
}

func TestNilDrainNeverDrains(t *testing.T) {
	var d *Drain
	d.Start()
	if d.Draining() {
		t.Fatal("nil drain reports draining")
	}
	h := healthMux(d.Check(time.Second, nil))
	if got := readyz(h); got != 200 {
		t.Fatalf("readyz: %d", got)
	}
}

func TestCheckFailsWhenTheProbeFails(t *testing.T) {
	d := NewDrain()
	boom := errors.New("down")
	h := healthMux(d.Check(time.Second, func(context.Context) error { return boom }))
	if got := readyz(h); got != 503 {
		t.Fatalf("readyz: %d", got)
	}
}

func TestCheckBoundsASlowProbe(t *testing.T) {
	d := NewDrain()
	probe := func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}
	h := healthMux(d.Check(20*time.Millisecond, probe))
	if got := readyz(h); got != 503 {
		t.Fatalf("readyz: %d", got)
	}
}

func TestCheckSkipsTheProbeWhileDraining(t *testing.T) {
	d := NewDrain()
	d.Start()
	called := false
	h := healthMux(d.Check(time.Second, func(context.Context) error {
		called = true
		return nil
	}))
	if got := readyz(h); got != 503 || called {
		t.Fatalf("status %d, probe called %v", got, called)
	}
}
