package pgstore_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/zellapay/zellapay/internal/webhooks"
	"github.com/zellapay/zellapay/internal/webhooks/webhookstest"
)

func TestOnlySubscribedEventsAreDelivered(t *testing.T) {
	w := newWorld(t)
	r := webhookstest.New(http.StatusOK)
	defer r.Close()
	w.endpoint(t, r, webhooks.EventCollectionFailed)
	w.announce(w.merchant.ID, webhooks.EventCollectionSucceeded)
	w.dispatch(t)
	if res := w.deliver(t); res.Attempted != 0 || r.Count() != 0 {
		t.Fatalf("an unsubscribed event must not be sent: %+v", res)
	}
	w.announce(w.merchant.ID, webhooks.EventCollectionFailed)
	w.dispatch(t)
	if res := w.deliver(t); res.Succeeded != 1 || r.Count() != 1 {
		t.Fatalf("the subscribed event must be sent: %+v", res)
	}
}

func TestMerchantsNeverReceiveEachOthersEvents(t *testing.T) {
	a, b := newWorld(t), newWorld(t)
	ra, rb := webhookstest.New(200), webhookstest.New(200)
	defer ra.Close()
	defer rb.Close()
	a.endpoint(t, ra)
	b.endpoint(t, rb)
	a.announce(a.merchant.ID, webhooks.EventCollectionSucceeded)
	a.dispatch(t)
	a.deliver(t)
	b.deliver(t)
	if ra.Count() != 1 || rb.Count() != 0 {
		t.Fatalf("merchant A received %d, merchant B received %d", ra.Count(), rb.Count())
	}
}

func TestAnEventWithNoEndpointsIsStoredAndCanBeReplayedLater(t *testing.T) {
	w := newWorld(t)
	obj := w.announce(w.merchant.ID, webhooks.EventCollectionSucceeded)
	w.dispatch(t)
	r := webhookstest.New(http.StatusOK)
	defer r.Close()
	w.endpoint(t, r)
	if w.deliver(t).Attempted != 0 {
		t.Fatal("nothing was scheduled before the endpoint existed")
	}
	n, err := w.svc.Replay(context.Background(), w.merchant.ID, w.eventID(t, obj))
	if err != nil || n != 1 {
		t.Fatalf("replay: %d, %v", n, err)
	}
	if res := w.deliver(t); res.Succeeded != 1 {
		t.Fatalf("got %+v", res)
	}
}

func TestReplayResendsWithAFreshSignature(t *testing.T) {
	w := newWorld(t)
	r := webhookstest.New(http.StatusOK)
	defer r.Close()
	ep := w.endpoint(t, r)
	obj := w.announce(w.merchant.ID, webhooks.EventCollectionSucceeded)
	w.dispatch(t)
	w.deliver(t)
	first := r.Last()

	w.advance(10 * 60 * 1e9)
	if _, err := w.svc.Replay(context.Background(), w.merchant.ID, w.eventID(t, obj)); err != nil {
		t.Fatal(err)
	}
	w.deliver(t)
	second := r.Last()
	if r.Count() != 2 || string(first.Body) != string(second.Body) ||
		first.Header.Get("ZellaPay-Event-Id") != second.Header.Get("ZellaPay-Event-Id") {
		t.Fatalf("replay must resend the same event: %d requests", r.Count())
	}
	if first.Header.Get(webhooks.SignatureHeader) == second.Header.Get(webhooks.SignatureHeader) {
		t.Fatal("a replay must be signed with a fresh timestamp")
	}
	if err := second.Verify(ep.Secret, w.clock.Now()); err != nil {
		t.Fatalf("the replayed signature must verify: %v", err)
	}
}

func TestReplayOfAnotherMerchantsEventIsRefused(t *testing.T) {
	a, b := newWorld(t), newWorld(t)
	obj := a.announce(a.merchant.ID, webhooks.EventCollectionSucceeded)
	a.dispatch(t)
	_, err := b.svc.Replay(context.Background(), b.merchant.ID, a.eventID(t, obj))
	if !errors.Is(err, webhooks.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}
