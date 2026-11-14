package rediscache_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/zellapay/zellapay/internal/platform/idempotency"
	"github.com/zellapay/zellapay/internal/platform/idempotency/rediscache"
)

var addr string

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()
	c, err := tcredis.Run(ctx, "redis:7")
	if err != nil {
		println("start redis:", err.Error())
		return 1
	}
	defer func() { _ = c.Terminate(ctx) }()
	uri, err := c.ConnectionString(ctx)
	if err != nil {
		println("redis uri:", err.Error())
		return 1
	}
	addr = uri
	return m.Run()
}

func newCache(t *testing.T) *rediscache.Cache {
	t.Helper()
	opt, err := redis.ParseURL(addr)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(opt)
	t.Cleanup(func() { _ = client.Close() })
	return rediscache.New(client)
}

func TestRoundTrip(t *testing.T) {
	c := newCache(t)
	ctx := context.Background()
	req := idempotency.Request{Scope: "m1", Key: "k1", Fingerprint: []byte{1, 2, 3}}
	resp := idempotency.Response{
		Status: 201, ContentType: "application/json", Body: []byte{0, 255, 7},
	}
	if err := c.Set(ctx, req, resp, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, found, err := c.Get(ctx, req)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if got.Response.Status != 201 || string(got.Response.Body) != string(resp.Body) ||
		string(got.Fingerprint) != string(req.Fingerprint) {
		t.Fatalf("round trip changed entry: %+v", got)
	}
}

func TestMissReportsNotFound(t *testing.T) {
	req := idempotency.Request{Scope: "m1", Key: "absent"}
	_, found, err := newCache(t).Get(context.Background(), req)
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
}

func TestEntriesExpire(t *testing.T) {
	c := newCache(t)
	ctx := context.Background()
	req := idempotency.Request{Scope: "m1", Key: "short"}
	if err := c.Set(ctx, req, idempotency.Response{}, 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	if _, found, _ := c.Get(ctx, req); found {
		t.Fatal("entry must expire")
	}
}
