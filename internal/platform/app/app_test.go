package app

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/platform/httpx"
)

type fixture struct {
	url     string
	drain   *httpx.Drain
	release chan struct{}
	entered chan struct{}
	cancel  context.CancelFunc
	done    chan error
}

func start(t *testing.T, plan Plan) *fixture {
	t.Helper()
	f := &fixture{
		drain:   httpx.NewDrain(),
		release: make(chan struct{}),
		entered: make(chan struct{}),
		done:    make(chan error, 1),
	}
	mux := http.NewServeMux()
	httpx.Health(mux, f.drain.Check(time.Second, nil))
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, _ *http.Request) {
		close(f.entered)
		<-f.release
		w.WriteHeader(http.StatusOK)
	})
	ln := listen(t)
	f.url = "http://" + ln.Addr().String()
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second}
	plan.Drain, plan.Listener = f.drain, ln
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	go func() { f.done <- Serve(ctx, log, srv, plan) }()
	t.Cleanup(cancel)
	return f
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

func status(t *testing.T, url string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

func TestServeDrainsBeforeStoppingTheListener(t *testing.T) {
	f := start(t, Plan{Timeout: 5 * time.Second, DrainDelay: 300 * time.Millisecond})
	if got := status(t, f.url+"/readyz"); got != 200 {
		t.Fatalf("before shutdown: %d", got)
	}
	f.cancel()
	<-f.drain.Started()
	if got := status(t, f.url+"/readyz"); got != 503 {
		t.Fatalf("while draining the listener must still answer 503, got %d", got)
	}
	if got := status(t, f.url+"/healthz"); got != 200 {
		t.Fatalf("liveness while draining: %d", got)
	}
	if err := <-f.done; err != nil {
		t.Fatal(err)
	}
}

func TestServeLetsInFlightRequestsFinish(t *testing.T) {
	f := start(t, Plan{Timeout: 5 * time.Second})
	result := make(chan int, 1)
	go func() { result <- status(t, f.url+"/slow") }()
	<-f.entered
	f.cancel()
	<-f.drain.Started()
	select {
	case err := <-f.done:
		t.Fatalf("Serve returned with a request in flight: %v", err)
	default:
	}
	close(f.release)
	if got := <-result; got != 200 {
		t.Fatalf("in-flight request: %d", got)
	}
	if err := <-f.done; err != nil {
		t.Fatal(err)
	}
}

func TestServeReturnsListenErrors(t *testing.T) {
	ln := listen(t)
	defer func() { _ = ln.Close() }()
	srv := &http.Server{Addr: ln.Addr().String(), ReadHeaderTimeout: time.Second}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	plan := Plan{Timeout: time.Second, Drain: httpx.NewDrain()}
	if err := Serve(context.Background(), log, srv, plan); err == nil {
		t.Fatal("expected the address-in-use error")
	}
}

func TestServeReportsAShutdownThatTimesOut(t *testing.T) {
	f := start(t, Plan{Timeout: 50 * time.Millisecond})
	go func() { _ = status(t, f.url+"/slow") }()
	<-f.entered
	f.cancel()
	if err := <-f.done; err == nil {
		t.Fatal("expected a shutdown timeout error")
	}
	close(f.release)
}
