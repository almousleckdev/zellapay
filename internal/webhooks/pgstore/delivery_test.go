package pgstore_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/webhooks"
	"github.com/zellapay/zellapay/internal/webhooks/webhookstest"
)

func TestADeliveredEventIsSignedAndComplete(t *testing.T) {
	w := newWorld(t)
	r := webhookstest.New(http.StatusOK)
	defer r.Close()
	ep := w.endpoint(t, r)
	obj := w.announce(w.merchant.ID, webhooks.EventCollectionSucceeded)

	w.dispatch(t)
	if res := w.deliver(t); res.Attempted != 1 || res.Succeeded != 1 {
		t.Fatalf("result %+v", res)
	}
	got := r.Last()
	if err := got.Verify(ep.Secret, w.clock.Now()); err != nil {
		t.Fatalf("the signature must verify with the secret given at creation: %v", err)
	}
	var env struct {
		ID, Type string
		Data     struct{ Object map[string]any }
	}
	if err := json.Unmarshal(got.Body, &env); err != nil {
		t.Fatal(err)
	}
	if env.ID != got.EventID() || env.Type != webhooks.EventCollectionSucceeded ||
		env.Data.Object["id"] != obj {
		t.Fatalf("unexpected payload %s", got.Body)
	}
	if got.Header.Get("ZellaPay-Delivery-Attempt") != "1" ||
		got.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("headers: %v", got.Header)
	}
}

func TestPublishingTheSameEntryTwiceMakesOneDelivery(t *testing.T) {
	w := newWorld(t)
	r := webhookstest.New(http.StatusOK)
	defer r.Close()
	w.endpoint(t, r)
	obj := w.announce(w.merchant.ID, webhooks.EventCollectionSucceeded)
	w.dispatch(t)
	// The same entry arrives again, as after a crash mid-publish.
	w.feed.add(w.entry(obj, webhooks.EventCollectionSucceeded))
	w.dispatch(t)
	w.deliver(t)
	w.deliver(t)
	if r.Count() != 1 {
		t.Fatalf("the merchant must receive the event once, got %d", r.Count())
	}
}

func TestFailedDeliveriesAreRetriedWithBackoff(t *testing.T) {
	w := newWorld(t)
	r := webhookstest.New(http.StatusInternalServerError, http.StatusBadGateway, http.StatusOK)
	defer r.Close()
	w.endpoint(t, r)
	w.announce(w.merchant.ID, webhooks.EventCollectionSucceeded)
	w.dispatch(t)

	w.deliver(t) // attempt 1 fails, retry in 30s
	if res := w.deliver(t); res.Attempted != 0 {
		t.Fatalf("nothing is due yet: %+v", res)
	}
	w.advance(webhooks.Backoff(1))
	w.deliver(t) // attempt 2 fails, retry in 60s
	w.advance(webhooks.Backoff(2) - time.Second)
	if res := w.deliver(t); res.Attempted != 0 {
		t.Fatalf("one second early must not retry: %+v", res)
	}
	w.advance(time.Second)
	if res := w.deliver(t); res.Succeeded != 1 {
		t.Fatalf("third attempt must succeed: %+v", res)
	}

	ds := w.deliveriesOf(t, r)
	log, _ := w.store.Attempts(context.Background(), ds[0].ID)
	want := []struct {
		status int
		err    string
	}{{500, "http_500"}, {502, "http_502"}, {200, ""}}
	if len(log) != 3 {
		t.Fatalf("attempt log: %+v", log)
	}
	for i, a := range log {
		if a.Number != i+1 || a.StatusCode != want[i].status || a.Error != want[i].err {
			t.Errorf("attempt %d = %+v", i+1, a)
		}
	}
	if ds[0].Status != webhooks.DeliverySucceeded {
		t.Fatalf("status = %s", ds[0].Status)
	}
}

func TestDeliveryGivesUpAfterTheRetryWindow(t *testing.T) {
	w := newWorld(t)
	r := webhookstest.New(http.StatusInternalServerError)
	defer r.Close()
	w.endpoint(t, r)
	start := w.clock.Now()
	w.announce(w.merchant.ID, webhooks.EventCollectionSucceeded)
	w.dispatch(t)

	for i := 0; i < 60; i++ {
		res := w.deliver(t)
		if res.Dead == 1 {
			break
		}
		w.advance(webhooks.Backoff(i + 1))
	}
	ds := w.deliveriesOf(t, r)
	if ds[0].Status != webhooks.DeliveryDead || ds[0].CompletedAt == nil {
		t.Fatalf("delivery %+v", ds[0])
	}
	if span := w.clock.Now().Sub(start); span > webhooks.RetryHorizon+time.Hour {
		t.Fatalf("kept retrying for %v", span)
	}
	before := r.Count()
	w.advance(24 * time.Hour)
	if w.deliver(t).Attempted != 0 || r.Count() != before {
		t.Fatal("a dead delivery must never be sent again")
	}
}

func TestUnreachableEndpointsAreRecordedWithoutLeakingDetail(t *testing.T) {
	w := newWorld(t)
	r := webhookstest.New(http.StatusOK)
	url := r.URL
	r.Close()
	if _, err := w.svc.CreateEndpoint(context.Background(), webhooks.CreateEndpointInput{
		MerchantID: w.merchant.ID, URL: url, Events: []string{webhooks.EventCollectionSucceeded},
	}); err != nil {
		t.Fatal(err)
	}
	obj := w.announce(w.merchant.ID, webhooks.EventCollectionSucceeded)
	w.dispatch(t)
	if res := w.deliver(t); res.Retrying != 1 {
		t.Fatalf("got %+v", res)
	}
	ev, _ := w.store.GetEvent(context.Background(), w.merchant.ID, w.eventID(t, obj))
	ds, _ := w.store.Deliveries(context.Background(), ev.ID)
	if ds[0].LastError != webhooks.CodeConnection || ds[0].LastStatus != 0 {
		t.Fatalf("delivery %+v", ds[0])
	}
}

var _ = http.StatusOK
