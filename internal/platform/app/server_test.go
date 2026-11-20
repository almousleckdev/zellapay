package app

import (
	"net/http"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/platform/config"
)

func TestNewServerUsesTheSharedConfiguration(t *testing.T) {
	cfg, err := config.Load("api", ":8123")
	if err != nil {
		t.Fatal(err)
	}
	h := http.NewServeMux()
	srv := NewServer(cfg, h)
	if srv.Addr != ":8123" || srv.Handler != h {
		t.Fatalf("addr %q handler %v", srv.Addr, srv.Handler)
	}
	got := [4]time.Duration{
		srv.ReadHeaderTimeout, srv.ReadTimeout, srv.WriteTimeout, srv.IdleTimeout,
	}
	want := [4]time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second, 60 * time.Second}
	if got != want {
		t.Fatalf("timeouts %v, want %v", got, want)
	}
}
