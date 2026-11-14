package pgstore_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/platform/idempotency"
	"github.com/zellapay/zellapay/internal/platform/idempotency/pgstore"
	"github.com/zellapay/zellapay/internal/platform/pgtest"
)

var (
	t0      = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	opts    = pgstore.Options{TTL: time.Hour, LockTTL: time.Minute}
	counter atomic.Int64
)

func newStore(t *testing.T) *pgstore.Store {
	t.Helper()
	return pgstore.New(pgtest.NewPool(t, testDSN), opts)
}

// newReq returns a request with a unique key so tests never collide.
func newReq(fp string) idempotency.Request {
	return idempotency.Request{
		Scope:       "m1",
		Key:         fmt.Sprintf("key-%d", counter.Add(1)),
		Fingerprint: padded(fp),
	}
}

func padded(s string) []byte {
	b := make([]byte, 32)
	copy(b, s)
	return b
}

func begin(
	t *testing.T, s *pgstore.Store, r idempotency.Request, at time.Time,
) idempotency.Decision {
	t.Helper()
	d, err := s.Begin(context.Background(), r, at)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestBeginCompleteThenReplay(t *testing.T) {
	s, r := newStore(t), newReq("a")
	if begin(t, s, r, t0).Kind != idempotency.Started {
		t.Fatal("first begin must start")
	}
	resp := idempotency.Response{
		Status: 201, ContentType: "application/json", Body: []byte(`{"id":1}`),
	}
	if err := s.Complete(context.Background(), r, resp, t0); err != nil {
		t.Fatal(err)
	}
	d := begin(t, s, r, t0.Add(time.Second))
	if d.Kind != idempotency.Replay || d.Response.Status != 201 ||
		string(d.Response.Body) != `{"id":1}` || d.Response.ContentType != "application/json" {
		t.Fatalf("unexpected replay %+v", d)
	}
}

func TestDifferentFingerprintConflicts(t *testing.T) {
	s, r := newStore(t), newReq("a")
	begin(t, s, r, t0)
	other := r
	other.Fingerprint = padded("b")
	if begin(t, s, other, t0).Kind != idempotency.Conflict {
		t.Fatal("different request under the same key must conflict")
	}
}

func TestInProgressIsInFlightUntilLockExpires(t *testing.T) {
	s, r := newStore(t), newReq("a")
	begin(t, s, r, t0)
	if begin(t, s, r, t0.Add(30*time.Second)).Kind != idempotency.InFlight {
		t.Fatal("a live claim must report in flight")
	}
	if begin(t, s, r, t0.Add(2*time.Minute)).Kind != idempotency.Started {
		t.Fatal("a stale claim for the same request may be taken over")
	}
}

func TestStaleClaimIsNotTakenOverByADifferentRequest(t *testing.T) {
	s, r := newStore(t), newReq("a")
	begin(t, s, r, t0)
	other := r
	other.Fingerprint = padded("b")
	if begin(t, s, other, t0.Add(2*time.Minute)).Kind != idempotency.Conflict {
		t.Fatal("misuse of a key must stay a conflict")
	}
}

func TestExpiredRecordAllowsAnyRequest(t *testing.T) {
	s, r := newStore(t), newReq("a")
	begin(t, s, r, t0)
	_ = s.Complete(context.Background(), r, idempotency.Response{Status: 200}, t0)
	other := r
	other.Fingerprint = padded("b")
	if begin(t, s, other, t0.Add(2*time.Hour)).Kind != idempotency.Started {
		t.Fatal("an expired key is free to reuse")
	}
}

func TestReleaseFreesTheKey(t *testing.T) {
	s, r := newStore(t), newReq("a")
	begin(t, s, r, t0)
	if err := s.Release(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if begin(t, s, r, t0).Kind != idempotency.Started {
		t.Fatal("a released key can be claimed again")
	}
}

func TestCompleteAfterTakeoverReportsLockLost(t *testing.T) {
	s, r := newStore(t), newReq("a")
	begin(t, s, r, t0)
	begin(t, s, r, t0.Add(2*time.Minute)) // second worker takes over
	_ = s.Release(context.Background(), r)
	err := s.Complete(context.Background(), r, idempotency.Response{Status: 200}, t0)
	if !errors.Is(err, idempotency.ErrLockLost) {
		t.Fatalf("got %v", err)
	}
}

func TestScopesAreIndependent(t *testing.T) {
	s, r := newStore(t), newReq("a")
	begin(t, s, r, t0)
	other := r
	other.Scope = "m2"
	if begin(t, s, other, t0).Kind != idempotency.Started {
		t.Fatal("another merchant's identical key is unrelated")
	}
}

func TestConcurrentBeginStartsExactlyOnce(t *testing.T) {
	s, r := newStore(t), newReq("a")
	var started, inflight atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch begin(t, s, r, t0).Kind {
			case idempotency.Started:
				started.Add(1)
			case idempotency.InFlight:
				inflight.Add(1)
			}
		}()
	}
	wg.Wait()
	if started.Load() != 1 || inflight.Load() != 15 {
		t.Fatalf("started=%d inflight=%d", started.Load(), inflight.Load())
	}
}

func TestPurgeExpiredRemovesOnlyExpiredRecords(t *testing.T) {
	s := newStore(t)
	old, fresh := newReq("a"), newReq("b")
	begin(t, s, old, t0)
	begin(t, s, fresh, t0.Add(3*time.Hour))
	n, err := s.PurgeExpired(context.Background(), t0.Add(2*time.Hour), 1000)
	if err != nil || n < 1 {
		t.Fatalf("purged %d, %v", n, err)
	}
	if begin(t, s, fresh, t0.Add(3*time.Hour)).Kind != idempotency.InFlight {
		t.Fatal("unexpired record must survive")
	}
}
