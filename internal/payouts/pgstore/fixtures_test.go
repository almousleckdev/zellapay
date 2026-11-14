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

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/ledger/ledgertest"
	ledgerstore "github.com/zellapay/zellapay/internal/ledger/pgstore"
	"github.com/zellapay/zellapay/internal/merchants"
	merchantstore "github.com/zellapay/zellapay/internal/merchants/pgstore"
	"github.com/zellapay/zellapay/internal/operators/router"
	"github.com/zellapay/zellapay/internal/operators/simulator"
	"github.com/zellapay/zellapay/internal/payouts"
	"github.com/zellapay/zellapay/internal/payouts/pgstore"
	"github.com/zellapay/zellapay/internal/platform/crypto"
	"github.com/zellapay/zellapay/internal/platform/ids"
	"github.com/zellapay/zellapay/internal/platform/pgtest"
)

const simDelay = 5 * time.Second

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

// world is a fully wired payouts service on real storage.
type world struct {
	svc      *payouts.Service
	store    *pgstore.Store
	ledger   *ledgerstore.Store
	merchant merchants.Merchant
	sim      *simulator.Simulator
	clock    *testClock
}

// newWorld returns a merchant whose payout fee is feeBPS and whose balance
// is fund, in the smallest unit.
func newWorld(t *testing.T, feeBPS int, fund int64) *world {
	t.Helper()
	pool := pgtest.NewPool(t, testDSN)
	clk := &testClock{now: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)}
	sim := simulator.New(simulator.Config{Delay: simDelay, Secret: []byte("sim-secret")}, clk)
	rt, err := router.New([]router.Rule{{Prefix: "+241", Operator: simulator.Code}}, sim)
	if err != nil {
		t.Fatal(err)
	}
	led := ledgerstore.New(pool)
	msvc := merchants.NewService(merchantstore.New(pool), clk, merchants.ModeTest)
	w := &world{ledger: led, sim: sim, clock: clk, store: pgstore.New(pool, sealer(t), indexer(t))}
	w.merchant = activeMerchant(t, msvc, feeBPS)
	w.svc = payouts.NewService(payouts.Deps{
		Repo: w.store, Terms: msvc, Router: rt, Ledger: led, Accounts: led,
		Clock: clk, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, payouts.DefaultConfig())
	if fund > 0 {
		w.fund(t, fund)
	}
	return w
}

func activeMerchant(t *testing.T, svc *merchants.Service, payoutFeeBPS int) merchants.Merchant {
	t.Helper()
	ctx := context.Background()
	m, err := svc.CreateMerchant(ctx, "Acme "+ids.New(""), "XAF")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetStatus(ctx, m.ID, merchants.StatusActive); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetPayoutFeeBPS(ctx, m.ID, payoutFeeBPS); err != nil {
		t.Fatal(err)
	}
	return m
}

func sealer(t *testing.T) *crypto.Sealer {
	t.Helper()
	s, err := crypto.NewSealer([]crypto.Key{{ID: "k1", Bytes: bytes.Repeat([]byte{7}, 32)}}, "k1")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func indexer(t *testing.T) *crypto.BlindIndexer {
	t.Helper()
	i, err := crypto.NewBlindIndexer(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return i
}

// fund credits the merchant's available balance, as settlement would.
func (w *world) fund(t *testing.T, amount int64) {
	t.Helper()
	ledgertest.FundMerchant(t, w.ledger, w.merchant.ID, amount)
}

// phone returns a number whose last two digits pick the simulator scenario.
func phone(suffix string) string { return "+241070000" + suffix }

func (w *world) input(suffix, key string, amount int64) payouts.CreateInput {
	return payouts.CreateInput{
		MerchantID: w.merchant.ID, IdempotencyKey: key, Reference: "PAYOUT-" + suffix,
		Phone: phone(suffix), Currency: "XAF", Amount: amount,
	}
}

func (w *world) create(t *testing.T, suffix string, amount int64) payouts.Payout {
	t.Helper()
	p, err := w.svc.Create(context.Background(), w.input(suffix, ids.New("key_"), amount))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return p
}

// deliverCallbacks hands every due simulator callback to the service.
func (w *world) deliverCallbacks(t *testing.T) int {
	t.Helper()
	due := w.sim.DueCallbacks()
	for _, cb := range due {
		req := httptest.NewRequestWithContext(context.Background(),
			http.MethodPost, "/callbacks/simulator", bytes.NewReader(cb.Body))
		req.Header.Set(simulator.SignatureHeader, cb.Signature)
		ev, err := w.sim.ParseCallback(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if ev.Ref.OurRef[:4] != "pay_" {
			continue // another test's collection
		}
		if err := w.svc.HandleEvent(context.Background(), simulator.Code, ev); err != nil {
			t.Fatalf("callback: %v", err)
		}
	}
	return len(due)
}

func (w *world) balance(t *testing.T, account string) int64 {
	t.Helper()
	got, err := w.ledger.Balance(context.Background(), account)
	if err != nil {
		t.Fatalf("balance of %s: %v", account, err)
	}
	return got
}

func (w *world) available(t *testing.T) int64 {
	return w.balance(t, ledger.MerchantAvailable(w.merchant.ID))
}

func (w *world) held(t *testing.T) int64 {
	return w.balance(t, ledger.MerchantHold(w.merchant.ID))
}

func (w *world) reload(t *testing.T, id string) payouts.Payout {
	t.Helper()
	p, err := w.svc.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
