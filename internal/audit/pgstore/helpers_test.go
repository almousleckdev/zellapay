package pgstore_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/audit/pgstore"
	"github.com/zellapay/zellapay/internal/platform/pgtest"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(1500 * time.Nanosecond) // sub-microsecond, like a real clock
	return c.now
}

type world struct {
	svc   *audit.Service
	store *pgstore.Store
	pool  *pgxpool.Pool
}

func newWorldOn(pool *pgxpool.Pool) *world {
	store := pgstore.New(pool)
	clk := &testClock{now: time.Date(2026, 11, 14, 12, 0, 0, 123456789, time.UTC)}
	return &world{svc: audit.NewService(store, clk), store: store, pool: pool}
}

// isolated uses a database of its own: for tests that damage the log.
func isolated(t *testing.T) *world { return newWorldOn(pgtest.Fresh(t, testDSN)) }

func entry(action string) audit.Entry {
	return audit.Entry{
		Actor: audit.Actor{Kind: audit.ActorStaff, ID: "stf_1"}, Action: action,
		Target: audit.Target{Kind: "merchant", ID: "mer_1"}, Reason: "fraud review FR-1",
		Change: map[string]string{"status": "suspended", "fee_bps": "150"},
		Source: audit.Source{IP: "203.0.113.7", UserAgent: "Mozilla/5.0 é", RequestID: "req_1"},
	}
}

func (w *world) record(t *testing.T, action string) audit.Record {
	t.Helper()
	r, err := w.svc.Record(context.Background(), entry(action))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (w *world) verify(t *testing.T) audit.VerifyResult {
	t.Helper()
	res, err := w.svc.Verify(context.Background(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func (w *world) exec(t *testing.T, sql string, args ...any) error {
	t.Helper()
	_, err := w.pool.Exec(context.Background(), sql, args...)
	return err
}
