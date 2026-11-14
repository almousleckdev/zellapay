package pgstore_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/zellapay/zellapay/internal/notify"
	"github.com/zellapay/zellapay/internal/notify/pgstore"
	"github.com/zellapay/zellapay/internal/platform/crypto"
	"github.com/zellapay/zellapay/internal/platform/pgtest"
)

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

type countingSender struct {
	sent atomic.Int32
	fail atomic.Bool
}

func (s *countingSender) Send(context.Context, notify.Delivery) error {
	s.sent.Add(1)
	if s.fail.Load() {
		return &notify.SendError{Code: notify.CodeUnavailable}
	}
	return nil
}

type world struct {
	svc    *notify.Service
	store  *pgstore.Store
	sender *countingSender
	clock  *testClock
}

func newWorld(t *testing.T) *world {
	t.Helper()
	pool := pgtest.NewPool(t, testDSN)
	box, err := crypto.NewSealer([]crypto.Key{{ID: "k1", Bytes: bytes.Repeat([]byte{5}, 32)}}, "k1")
	if err != nil {
		t.Fatal(err)
	}
	w := &world{
		store: pgstore.New(pool), sender: &countingSender{},
		clock: &testClock{now: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)},
	}
	w.svc, err = notify.NewService(notify.Deps{
		Repo: w.store, Senders: map[notify.Channel]notify.Sender{notify.Email: w.sender},
		Box: box, Clock: w.clock, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, notify.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// canaryLink is a link that must never appear in storage in the clear.
const canaryLink = "https://app.example/verify/canary-token"

func message(key string) notify.Message {
	return notify.Message{
		To: "owner@example.com", Template: "email_verification", Locale: "fr",
		Data:      map[string]string{"hours": "24", "link": canaryLink},
		DedupeKey: key,
	}
}

func (w *world) deliver(t *testing.T) notify.DeliverResult {
	t.Helper()
	res, err := w.svc.DeliverDue(context.Background(), 200)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestNothingSensitiveIsStoredInTheClear(t *testing.T) {
	w := newWorld(t)
	id, _, err := w.svc.Enqueue(context.Background(), message("verify:a"))
	if err != nil {
		t.Fatal(err)
	}
	var enc, masked string
	const query = "SELECT payload_enc, recipient_masked FROM notifications WHERE id = $1"
	err = pgtest.NewPool(t, testDSN).QueryRow(context.Background(), query, id).Scan(&enc, &masked)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"owner@example.com", "canary-token", "app.example"} {
		if strings.Contains(enc, secret) || strings.Contains(masked, secret) {
			t.Fatalf("the database reveals %q", secret)
		}
	}
	if masked != "o***@example.com" || !strings.HasPrefix(enc, "v1:k1:") {
		t.Fatalf("masked=%q enc=%q", masked, enc)
	}
}

func TestDeliveryErasesThePayloadInTheDatabase(t *testing.T) {
	w := newWorld(t)
	id, _, _ := w.svc.Enqueue(context.Background(), message("verify:b"))
	if res := w.deliver(t); res.Sent < 1 {
		t.Fatalf("got %+v", res)
	}
	rec, err := w.store.Get(context.Background(), id)
	if err != nil || rec.Status != notify.StatusSent || !rec.PayloadErased || rec.SentAt == nil {
		t.Fatalf("record %+v, %v", rec, err)
	}
}

func TestADuplicateKeyStoresNothingTwice(t *testing.T) {
	w := newWorld(t)
	_, first, _ := w.svc.Enqueue(context.Background(), message("verify:c"))
	_, second, err := w.svc.Enqueue(context.Background(), message("verify:c"))
	if err != nil || !first || second {
		t.Fatalf("first=%v second=%v err=%v", first, second, err)
	}
}

func TestRetriesFollowTheBackoffInTheDatabase(t *testing.T) {
	w := newWorld(t)
	w.sender.fail.Store(true)
	id, _, _ := w.svc.Enqueue(context.Background(), message("verify:d"))
	w.deliver(t)
	rec, _ := w.store.Get(context.Background(), id)
	retrying := rec.Status == notify.StatusPending && rec.Attempts == 1
	if !retrying || rec.LastError != notify.CodeUnavailable {
		t.Fatalf("record %+v", rec)
	}
	before := w.sender.sent.Load()
	w.deliver(t)
	if w.sender.sent.Load() != before {
		t.Fatal("must wait for the backoff")
	}
	w.sender.fail.Store(false)
	w.clock.advance(notify.Backoff(1))
	w.deliver(t)
	rec, _ = w.store.Get(context.Background(), id)
	if rec.Status != notify.StatusSent || rec.Attempts != 2 {
		t.Fatalf("record %+v", rec)
	}
}

func TestConcurrentWorkersNeverSendAMessageTwice(t *testing.T) {
	w := newWorld(t)
	const n = 20
	for i := 0; i < n; i++ {
		m := message("")
		m.To = "user" + string(rune('a'+i)) + "@example.com"
		if _, _, err := w.svc.Enqueue(context.Background(), m); err != nil {
			t.Fatal(err)
		}
	}
	w.deliver(t) // drain anything earlier tests left behind
	before := w.sender.sent.Load()
	for i := 0; i < n; i++ {
		m := message("")
		m.To = "other" + string(rune('a'+i)) + "@example.com"
		_, _, _ = w.svc.Enqueue(context.Background(), m)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = w.svc.DeliverDue(context.Background(), 200)
		}()
	}
	wg.Wait()
	if got := w.sender.sent.Load() - before; got != n {
		t.Fatalf("%d sends for %d messages", got, n)
	}
}

// A message queued inside a transaction exists only if the transaction
// commits: a sign-up that fails must not send a verification email.
func TestAMessageQueuedInATransactionFollowsItsFate(t *testing.T) {
	w := newWorld(t)
	pool := pgtest.NewPool(t, testDSN)
	prepare := func(key string) notify.Prepared {
		p, err := w.svc.Prepare(message(key))
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	rolledBack, committed := prepare("tx:rollback"), prepare("tx:commit")
	ctx := context.Background()

	tx, _ := pool.Begin(ctx)
	if _, err := w.store.InsertIn(ctx, tx, rolledBack); err != nil {
		t.Fatal(err)
	}
	_ = tx.Rollback(ctx)
	if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		_, err := w.store.InsertIn(ctx, tx, committed)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.store.Get(ctx, rolledBack.ID); err == nil {
		t.Fatal("a rolled-back message must not exist")
	}
	if _, err := w.store.Get(ctx, committed.ID); err != nil {
		t.Fatalf("a committed message must exist: %v", err)
	}
}

func TestFinalMessagesCannotBeRevived(t *testing.T) {
	w := newWorld(t)
	id, _, _ := w.svc.Enqueue(context.Background(), message("verify:e"))
	w.deliver(t)
	pool := pgtest.NewPool(t, testDSN)
	for _, stmt := range []string{
		"UPDATE notifications SET status = 'pending' WHERE id = $1",
		"UPDATE notifications SET payload_enc = 'x' WHERE id = $1",
	} {
		if _, err := pool.Exec(context.Background(), stmt, id); err == nil {
			t.Errorf("%q must be rejected", stmt)
		}
	}
}
