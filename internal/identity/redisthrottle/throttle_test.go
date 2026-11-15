package redisthrottle_test

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/identity/redisthrottle"
	"github.com/zellapay/zellapay/internal/platform/crypto"
	"github.com/zellapay/zellapay/internal/platform/redistest"
)

var addr string

func TestMain(m *testing.M) {
	os.Exit(redistest.Main(m, func(url string) { addr = url }))
}

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time          { return c.now }
func (c *testClock) advance(d time.Duration) { c.now = c.now.Add(d) }

type rig struct {
	l     *redisthrottle.Limiter
	clock *testClock
	rdb   *redis.Client
}

func newRig(t *testing.T) *rig {
	t.Helper()
	opt, err := redis.ParseURL(addr)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opt)
	t.Cleanup(func() { _ = rdb.Close() })
	rdb.FlushAll(context.Background())
	idx, err := crypto.NewBlindIndexer(bytes.Repeat([]byte{4}, 32))
	if err != nil {
		t.Fatal(err)
	}
	clk := &testClock{now: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)}
	return &rig{
		l: redisthrottle.New(rdb, idx, clk, redisthrottle.DefaultPolicy()), clock: clk, rdb: rdb,
	}
}

func (r *rig) fail(t *testing.T, a identity.Attempt, times int) {
	t.Helper()
	for range times {
		if err := r.l.Failed(context.Background(), a); err != nil {
			t.Fatal(err)
		}
	}
}

func (r *rig) wait(t *testing.T, a identity.Attempt) time.Duration {
	t.Helper()
	d, err := r.l.Wait(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func login(account, ip string) identity.Attempt {
	return identity.Attempt{Realm: identity.RealmPassword, Account: account, IP: ip}
}

func TestAccountDelaysGrowAfterFiveFailures(t *testing.T) {
	r := newRig(t)
	a := login("jane@example.com", "")
	r.fail(t, a, 4)
	if d := r.wait(t, a); d != 0 {
		t.Fatalf("after 4 failures wait = %v, want none", d)
	}
	for _, want := range []time.Duration{
		time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute,
		15 * time.Minute, 15 * time.Minute,
	} {
		r.fail(t, a, 1)
		if d := r.wait(t, a); d != want {
			t.Fatalf("wait = %v, want %v", d, want)
		}
		r.clock.advance(want)
		if d := r.wait(t, a); d != 0 {
			t.Fatalf("after the delay, wait = %v, want none", d)
		}
	}
}

func TestSuccessClearsTheAccount(t *testing.T) {
	r := newRig(t)
	a := login("jane@example.com", "")
	r.fail(t, a, 6)
	if err := r.l.Succeeded(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if d := r.wait(t, a); d != 0 {
		t.Fatalf("wait = %v after success", d)
	}
	r.fail(t, a, 4)
	if d := r.wait(t, a); d != 0 {
		t.Fatal("the count did not start again from zero")
	}
}

func TestAccountsAndRealmsAreIndependent(t *testing.T) {
	r := newRig(t)
	r.fail(t, login("jane@example.com", ""), 6)
	if d := r.wait(t, login("other@example.com", "")); d != 0 {
		t.Fatal("another account was slowed down")
	}
	mfa := identity.Attempt{Realm: identity.RealmMFA, Account: "jane@example.com"}
	if d := r.wait(t, mfa); d != 0 {
		t.Fatal("the code check shares the password counter")
	}
}

func TestOneAddressGuessingManyAccountsIsStopped(t *testing.T) {
	r := newRig(t)
	for i := range 10 {
		r.fail(t, login(string(rune('a'+i))+"@example.com", "203.0.113.9"), 1)
	}
	d := r.wait(t, login("fresh@example.com", "203.0.113.9"))
	if d <= 0 || d > 10*time.Minute {
		t.Fatalf("wait = %v, want a delay within the window", d)
	}
	if d := r.wait(t, login("fresh@example.com", "203.0.113.10")); d != 0 {
		t.Fatal("a different address was slowed down")
	}
	r.clock.advance(11 * time.Minute)
	if d := r.wait(t, login("fresh@example.com", "203.0.113.9")); d != 0 {
		t.Fatalf("wait = %v after the window passed", d)
	}
}

func TestOneAddressRepeatingFailuresIsStopped(t *testing.T) {
	r := newRig(t)
	r.fail(t, login("jane@example.com", "203.0.113.9"), 20)
	if d := r.wait(t, login("jane@example.com", "203.0.113.9")); d <= 0 {
		t.Fatal("twenty failures from one address were not stopped")
	}
}

func TestRedisHoldsNeitherEmailsNorAddresses(t *testing.T) {
	r := newRig(t)
	r.fail(t, login("jane@example.com", "203.0.113.9"), 6)
	ctx := context.Background()
	keys, err := r.rdb.Keys(ctx, "*").Result()
	if err != nil || len(keys) == 0 {
		t.Fatalf("keys = %v, %v", keys, err)
	}
	for _, k := range keys {
		for _, secret := range []string{"jane", "example", "203.0.113"} {
			if bytes.Contains([]byte(k), []byte(secret)) {
				t.Errorf("key %q contains %q", k, secret)
			}
		}
	}
}
