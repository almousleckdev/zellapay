package wiring

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func blackhole(t *testing.T) string {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return ln.Addr().String()
}

func unresponsiveContainer(t *testing.T) *Container {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), "postgres://u:p@"+blackhole(t)+"/db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	rc := redis.NewClient(&redis.Options{Addr: blackhole(t)})
	t.Cleanup(func() { _ = rc.Close() })
	return &Container{Pool: pool, Redis: rc}
}

func TestReadyGivesUpWhenPostgresAndRedisStopAnswering(t *testing.T) {
	c := unresponsiveContainer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := c.Ready(ctx)
	if err == nil {
		t.Fatal("expected an error")
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("Ready took %v despite a 150ms deadline", took)
	}
	for _, name := range []string{"database", "redis"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
}
