package pgstore_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/collections/pgstore"
	ledgerstore "github.com/zellapay/zellapay/internal/ledger/pgstore"
	"github.com/zellapay/zellapay/internal/merchants"
	merchantstore "github.com/zellapay/zellapay/internal/merchants/pgstore"
	"github.com/zellapay/zellapay/internal/operators/router"
	"github.com/zellapay/zellapay/internal/operators/simulator"
	"github.com/zellapay/zellapay/internal/platform/crypto"
	"github.com/zellapay/zellapay/internal/platform/ids"
	"github.com/zellapay/zellapay/internal/platform/pgtest"
)

// testClock is shared by the simulator and the service so a test controls
// when the operator "answers".
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

// world is a fully wired collections service on real storage.
type world struct {
	svc      *collections.Service
	store    *pgstore.Store
	ledger   *ledgerstore.Store
	merchant merchants.Merchant
	sim      *simulator.Simulator
	clock    *testClock
}

const simDelay = 5 * time.Second

func newWorld(t *testing.T, feeBPS int) *world {
	t.Helper()
	pool := pgtest.NewPool(t, testDSN)
	clk := &testClock{now: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)}
	sim := simulator.New(simulator.Config{Delay: simDelay, Secret: []byte("sim-secret")}, clk)
	rt, err := router.New([]router.Rule{{Prefix: "+241", Operator: simulator.Code}}, sim)
	if err != nil {
		t.Fatal(err)
	}
	led := ledgerstore.New(pool)
	store := pgstore.New(pool, newSealer(t), newIndexer(t))
	msvc := merchants.NewService(merchantstore.New(pool), clk, merchants.ModeTest)
	w := &world{store: store, ledger: led, sim: sim, clock: clk}
	w.merchant = activeMerchant(t, msvc, feeBPS)
	w.svc = collections.NewService(collections.Deps{
		Repo: store, Terms: msvc, Router: rt, Ledger: led, Accounts: led,
		Clock: clk, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, collections.DefaultConfig())
	return w
}

func activeMerchant(t *testing.T, svc *merchants.Service, bps int) merchants.Merchant {
	t.Helper()
	ctx := context.Background()
	m, err := svc.CreateMerchant(ctx, "Acme "+ids.New(""), "XAF")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetStatus(ctx, m.ID, merchants.StatusActive); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetFeeBPS(ctx, m.ID, bps); err != nil {
		t.Fatal(err)
	}
	return m
}

func newSealer(t *testing.T) *crypto.Sealer {
	t.Helper()
	s, err := crypto.NewSealer([]crypto.Key{{ID: "k1", Bytes: bytes.Repeat([]byte{7}, 32)}}, "k1")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newIndexer(t *testing.T) *crypto.BlindIndexer {
	t.Helper()
	i, err := crypto.NewBlindIndexer(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return i
}

// phone returns a Gabonese test number whose last two digits pick the
// simulator scenario.
func phone(suffix string) string { return "+241070000" + suffix }

func (w *world) create(t *testing.T, suffix string) collections.Collection {
	t.Helper()
	c, err := w.svc.Create(context.Background(), w.input(suffix, ids.New("key_")))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func (w *world) input(suffix, key string) collections.CreateInput {
	return collections.CreateInput{
		MerchantID: w.merchant.ID, IdempotencyKey: key, Reference: "ORDER-" + suffix,
		Phone: phone(suffix), Currency: "XAF", Amount: 10000,
	}
}

// deliverCallbacks posts every due simulator callback to the service.
func (w *world) deliverCallbacks(t *testing.T) int {
	t.Helper()
	due := w.sim.DueCallbacks()
	for _, cb := range due {
		req := httptest.NewRequestWithContext(context.Background(),
			http.MethodPost, "/callbacks/simulator", bytes.NewReader(cb.Body))
		req.Header.Set(simulator.SignatureHeader, cb.Signature)
		ev, err := w.sim.ParseCallback(context.Background(), req)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if err := w.svc.HandleEvent(context.Background(), simulator.Code, ev); err != nil {
			t.Fatalf("callback: %v", err)
		}
	}
	return len(due)
}
