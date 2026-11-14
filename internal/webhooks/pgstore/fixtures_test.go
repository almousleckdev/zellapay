package pgstore_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/merchants"
	merchantstore "github.com/zellapay/zellapay/internal/merchants/pgstore"
	"github.com/zellapay/zellapay/internal/platform/crypto"
	"github.com/zellapay/zellapay/internal/platform/ids"
	"github.com/zellapay/zellapay/internal/platform/pgtest"
	"github.com/zellapay/zellapay/internal/webhooks"
	"github.com/zellapay/zellapay/internal/webhooks/httpsender"
	"github.com/zellapay/zellapay/internal/webhooks/pgstore"
	"github.com/zellapay/zellapay/internal/webhooks/webhookstest"
)

// receiver is the merchant side of the conversation.
type receiver = webhookstest.Receiver

var dev = webhooks.Policy{AllowPrivate: true, AllowHTTP: true}

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// fakeFeed is an in-memory outbox with the real claim semantics.
type fakeFeed struct {
	mu      sync.Mutex
	entries []webhooks.OutboxEntry
}

func (f *fakeFeed) add(e webhooks.OutboxEntry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append(f.entries, e)
}

func (f *fakeFeed) Claim(
	ctx context.Context, _ time.Time, limit int, handle webhooks.ClaimHandler,
) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := min(limit, len(f.entries))
	if n == 0 {
		return 0, nil
	}
	if err := handle(ctx, f.entries[:n]); err != nil {
		return 0, err
	}
	f.entries = f.entries[n:]
	return n, nil
}

// fakeSource answers with canned objects, keyed by object ID.
type fakeSource struct{ objects map[string]string }

func (f *fakeSource) Object(
	_ context.Context, e webhooks.OutboxEntry,
) (string, json.RawMessage, error) {
	merchant, ok := f.objects[e.ObjectID]
	if !ok {
		return "", nil, webhooks.ErrNotFound
	}
	return merchant, json.RawMessage(`{"id":"` + e.ObjectID + `","status":"succeeded"}`), nil
}

// world is the webhooks service on real storage.
type world struct {
	svc      *webhooks.Service
	store    *pgstore.Store
	feed     *fakeFeed
	source   *fakeSource
	clock    *testClock
	merchant merchants.Merchant
}

func newWorld(t *testing.T) *world {
	t.Helper()
	pool := pgtest.NewPool(t, testDSN)
	clk := &testClock{now: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)}
	w := &world{
		store:  pgstore.New(pool),
		feed:   &fakeFeed{},
		source: &fakeSource{objects: map[string]string{}},
		clock:  clk,
	}
	msvc := merchants.NewService(merchantstore.New(pool), clk, merchants.ModeTest)
	m, err := msvc.CreateMerchant(context.Background(), "Acme "+ids.New(""), "XAF")
	if err != nil {
		t.Fatal(err)
	}
	w.merchant = m
	w.svc = webhooks.NewService(webhooks.Deps{
		Repo: w.store, Sender: httpsender.New(dev, 2*time.Second),
		Streams: []webhooks.Stream{{Feed: w.feed, Source: w.source}},
		Box:     newSealer(t, 7), Clock: clk, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, webhooks.Config{Policy: dev, MaxEndpoints: 10, Lease: time.Minute, Concurrency: 4})
	return w
}

func newSealer(t *testing.T, fill byte) *crypto.Sealer {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = fill
	}
	s, err := crypto.NewSealer([]crypto.Key{{ID: "k1", Bytes: key}}, "k1")
	if err != nil {
		t.Fatal(err)
	}
	return s
}
