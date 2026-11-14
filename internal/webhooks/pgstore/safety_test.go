package pgstore_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/platform/pgtest"
	"github.com/zellapay/zellapay/internal/webhooks"
	"github.com/zellapay/zellapay/internal/webhooks/httpsender"
	"github.com/zellapay/zellapay/internal/webhooks/webhookstest"
)

// newProductionWorld returns a service with the strict production policy.
func newProductionWorld(t *testing.T, w *world) *webhooks.Service {
	t.Helper()
	return webhooks.NewService(webhooks.Deps{
		Repo: w.store, Sender: httpsender.New(webhooks.Policy{}, time.Second), Feed: w.feed,
		Source: w.source, Box: newSealer(t, 7), Clock: w.clock,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, webhooks.DefaultConfig())
}

func TestConcurrentWorkersNeverSendTheSameDeliveryTwice(t *testing.T) {
	w := newWorld(t)
	r := webhookstest.New(http.StatusOK)
	defer r.Close()
	for i := 0; i < 6; i++ {
		w.endpoint(t, r)
	}
	w.announce(w.merchant.ID, webhooks.EventCollectionSucceeded)
	w.dispatch(t)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = w.svc.DeliverDue(context.Background(), 100)
		}()
	}
	wg.Wait()
	if r.Count() != 6 {
		t.Fatalf("six endpoints must each get the event once, got %d requests", r.Count())
	}
}

func TestAClaimedDeliveryIsHeldUntilItsLeaseExpires(t *testing.T) {
	w := newWorld(t)
	r := webhookstest.New(http.StatusOK)
	defer r.Close()
	w.endpoint(t, r)
	w.announce(w.merchant.ID, webhooks.EventCollectionSucceeded)
	w.dispatch(t)
	// A worker claims the delivery and then dies before sending.
	claimed, err := w.store.ClaimDue(context.Background(), w.clock.Now(), 10, time.Minute)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %d, %v", len(claimed), err)
	}
	if w.deliver(t).Attempted != 0 {
		t.Fatal("a leased delivery must not be sent by another worker")
	}
	w.advance(time.Minute + time.Second)
	if res := w.deliver(t); res.Succeeded != 1 {
		t.Fatalf("after the lease expires it must be retried: %+v", res)
	}
}

func TestAnUnreadableSecretIsAFailedAttemptNotALostEvent(t *testing.T) {
	w := newWorld(t)
	r := webhookstest.New(http.StatusOK)
	defer r.Close()
	w.endpoint(t, r)
	w.announce(w.merchant.ID, webhooks.EventCollectionSucceeded)
	w.dispatch(t)
	wrongKey := webhooks.NewService(webhooks.Deps{
		Repo: w.store, Sender: httpsender.New(dev, time.Second), Feed: w.feed, Source: w.source,
		Box: newSealer(t, 99), Clock: w.clock, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, webhooks.Config{Policy: dev, MaxEndpoints: 10, Lease: time.Minute, Concurrency: 1})
	res, err := wrongKey.DeliverDue(context.Background(), 10)
	if err != nil || res.Retrying != 1 || r.Count() != 0 {
		t.Fatalf("got %+v, %v, %d requests", res, err, r.Count())
	}
	w.advance(time.Minute)
	if res := w.deliver(t); res.Succeeded != 1 {
		t.Fatalf("with the right key the event is still delivered: %+v", res)
	}
}

func TestStoredEventsAndAttemptsCannotBeChanged(t *testing.T) {
	w := newWorld(t)
	r := webhookstest.New(http.StatusOK)
	defer r.Close()
	w.endpoint(t, r)
	obj := w.announce(w.merchant.ID, webhooks.EventCollectionSucceeded)
	w.dispatch(t)
	w.deliver(t)
	id := w.eventID(t, obj)
	pool := pgtest.NewPool(t, testDSN)
	for _, stmt := range []string{
		"UPDATE webhook_events SET payload = 'x' WHERE id = '" + id + "'",
		"DELETE FROM webhook_events WHERE id = '" + id + "'",
		"UPDATE webhook_attempts SET status_code = 500",
		"DELETE FROM webhook_attempts",
	} {
		if _, err := pool.Exec(context.Background(), stmt); err == nil {
			t.Errorf("%q must be rejected", stmt)
		}
	}
}
