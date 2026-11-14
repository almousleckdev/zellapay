package pgstore_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/platform/pgtest"
)

// claimAll drains the outbox and returns what it offered.
func (w *world) claimAll(t *testing.T) []collections.OutboxEntry {
	t.Helper()
	var got []collections.OutboxEntry
	_, err := w.store.ClaimOutbox(context.Background(), w.clock.Now(), 1000,
		func(_ context.Context, entries []collections.OutboxEntry) error {
			got = append(got, entries...)
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func find(entries []collections.OutboxEntry, collectionID string) []collections.OutboxEntry {
	var out []collections.OutboxEntry
	for _, e := range entries {
		if e.CollectionID == collectionID {
			out = append(out, e)
		}
	}
	return out
}

func TestOnlyFinalStatusesWriteToTheOutbox(t *testing.T) {
	w := newWorld(t, 0)
	open := w.create(t, "00")     // pending: not final
	failed := w.create(t, "08")   // rejected at initiation: final
	w.claimAll(t)                 // drain anything earlier tests left behind
	w.clock.advance(simDelay + 1) // let the pending one finish
	w.deliverCallbacks(t)

	got := w.claimAll(t)
	if e := find(got, failed.ID); len(e) != 0 {
		t.Fatalf("already published earlier, got %v", e)
	}
	done := find(got, open.ID)
	if len(done) != 1 || done[0].EventType != collections.OutboxSucceeded {
		t.Fatalf("succeeded collection must announce itself once: %v", done)
	}
}

func TestEachFinalStatusHasItsOwnEventType(t *testing.T) {
	w := newWorld(t, 0)
	w.claimAll(t)
	rejected := w.create(t, "08")
	declined := w.create(t, "01")
	expired := w.create(t, "09")
	w.clock.advance(simDelay + collections.DefaultConfig().PendingCheckAfter)
	w.deliverCallbacks(t)
	if _, err := w.svc.Sweep(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	got := w.claimAll(t)
	want := map[string]string{
		rejected.ID: collections.OutboxFailed,
		declined.ID: collections.OutboxFailed,
		expired.ID:  collections.OutboxExpired,
	}
	for id, typ := range want {
		if e := find(got, id); len(e) != 1 || e[0].EventType != typ {
			t.Errorf("%s: got %v, want one %s", id, e, typ)
		}
	}
}

func TestAFailedHandlerLeavesEntriesToBeOfferedAgain(t *testing.T) {
	w := newWorld(t, 0)
	w.claimAll(t)
	c := w.create(t, "08")
	boom := errors.New("downstream unavailable")
	_, err := w.store.ClaimOutbox(context.Background(), w.clock.Now(), 100,
		func(context.Context, []collections.OutboxEntry) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	if e := find(w.claimAll(t), c.ID); len(e) != 1 {
		t.Fatalf("the entry must be offered again after a failure, got %v", e)
	}
	if e := find(w.claimAll(t), c.ID); len(e) != 0 {
		t.Fatalf("a published entry must not be offered again, got %v", e)
	}
}

func TestConcurrentClaimersNeverShareAnEntry(t *testing.T) {
	w := newWorld(t, 0)
	w.claimAll(t)
	for i := 0; i < 6; i++ {
		w.create(t, "08")
	}
	var mu sync.Mutex
	seen := map[int64]int{}
	var wg sync.WaitGroup
	release := make(chan struct{})
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = w.store.ClaimOutbox(context.Background(), w.clock.Now(), 2,
				func(_ context.Context, entries []collections.OutboxEntry) error {
					mu.Lock()
					for _, e := range entries {
						seen[e.ID]++
					}
					mu.Unlock()
					<-release // hold the lock so the others must skip these rows
					return nil
				})
		}()
	}
	time.Sleep(300 * time.Millisecond)
	close(release)
	wg.Wait()
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("entry %d was offered %d times at once", id, n)
		}
	}
	if len(seen) != 6 {
		t.Fatalf("three claimers of two should cover six entries, got %d", len(seen))
	}
}

func TestOutboxRowsCannotBeDeletedOrRewritten(t *testing.T) {
	w := newWorld(t, 0)
	c := w.create(t, "08")
	pool := pgtest.NewPool(t, testDSN)
	for _, stmt := range []string{
		"DELETE FROM collection_outbox WHERE collection_id = $1",
		"UPDATE collection_outbox SET event_type = 'collection.succeeded' WHERE collection_id = $1",
		"UPDATE collection_outbox SET collection_id = 'x' WHERE collection_id = $1",
	} {
		if _, err := pool.Exec(context.Background(), stmt, c.ID); err == nil {
			t.Errorf("%q must be rejected", stmt)
		}
	}
}
