package pgstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/platform/ids"
	"github.com/zellapay/zellapay/internal/webhooks"
)

// endpoint registers an endpoint pointing at r and returns it with its secret.
func (w *world) endpoint(t *testing.T, r *receiver, events ...string) webhooks.CreatedEndpoint {
	t.Helper()
	if len(events) == 0 {
		events = []string{webhooks.EventCollectionSucceeded}
	}
	ep, err := w.svc.CreateEndpoint(context.Background(), webhooks.CreateEndpointInput{
		MerchantID: w.merchant.ID, URL: r.URL, Events: events,
	})
	if err != nil {
		t.Fatal(err)
	}
	return ep
}

// announce puts one "collection succeeded" entry in the outbox for merchant
// and returns the object ID.
func (w *world) announce(merchantID, eventType string) string {
	obj := ids.New("col_")
	w.source.objects[obj] = merchantID
	w.feed.add(w.entry(obj, eventType))
	return obj
}

func (w *world) dispatch(t *testing.T) {
	t.Helper()
	if _, err := w.svc.Dispatch(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
}

func (w *world) deliver(t *testing.T) webhooks.DeliverResult {
	t.Helper()
	res, err := w.svc.DeliverDue(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// deliveries returns every delivery of the event announced for object.
func (w *world) deliveriesOf(t *testing.T, r *receiver) []webhooks.Delivery {
	t.Helper()
	ev, ok := w.eventFrom(t, r)
	if !ok {
		return nil
	}
	ds, err := w.store.Deliveries(context.Background(), ev)
	if err != nil {
		t.Fatal(err)
	}
	return ds
}

// eventFrom reads the event ID out of the last request a receiver got.
func (w *world) eventFrom(t *testing.T, r *receiver) (string, bool) {
	t.Helper()
	if r.Count() == 0 {
		return "", false
	}
	return r.Last().Header.Get("ZellaPay-Event-Id"), true
}

func (w *world) advance(d time.Duration) { w.clock.advance(d) }

// eventID finds the event stored for an object.
func (w *world) eventID(t *testing.T, object string) string {
	t.Helper()
	ev, err := w.store.SaveEvent(context.Background(), webhooks.Event{
		ID: "evt_probe", MerchantID: w.merchant.ID, Type: webhooks.EventCollectionSucceeded,
		ObjectID: object, Payload: []byte("{}"), CreatedAt: w.clock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return ev.ID
}

// entry builds an outbox entry for an object, stamped with the current time.
func (w *world) entry(object, eventType string) webhooks.OutboxEntry {
	return webhooks.OutboxEntry{ID: 1, ObjectID: object, Type: eventType, CreatedAt: w.clock.Now()}
}
