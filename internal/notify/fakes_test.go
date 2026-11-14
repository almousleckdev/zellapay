package notify

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/platform/crypto"
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

// memRepo is an in-memory outbox with the same contract as the real one.
type memRepo struct {
	mu     sync.Mutex
	rows   map[string]*row
	order  []string
	dedupe map[string]bool
}

type row struct {
	p    Prepared
	rec  Record
	next time.Time
}

func newMemRepo() *memRepo {
	return &memRepo{rows: map[string]*row{}, dedupe: map[string]bool{}}
}

func (m *memRepo) Insert(_ context.Context, p Prepared) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p.DedupeKey != "" {
		if m.dedupe[p.DedupeKey] {
			return false, nil
		}
		m.dedupe[p.DedupeKey] = true
	}
	m.rows[p.ID] = &row{p: p, next: p.CreatedAt, rec: Record{
		ID: p.ID, Channel: p.Channel, Template: p.Template, RecipientMasked: p.RecipientMasked,
		Status: StatusPending, CreatedAt: p.CreatedAt,
	}}
	m.order = append(m.order, p.ID)
	return true, nil
}

func (m *memRepo) ClaimDue(
	_ context.Context, now time.Time, limit int, lease time.Duration,
) ([]Queued, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Queued
	for _, id := range m.order {
		r := m.rows[id]
		if r.rec.Status != StatusPending || r.next.After(now) || len(out) >= limit {
			continue
		}
		r.next = now.Add(lease)
		out = append(out, Queued{
			ID: id, Channel: r.p.Channel, Template: r.p.Template, Locale: r.p.Locale,
			PayloadSealed: r.p.PayloadSealed, Attempts: r.rec.Attempts, RetryUntil: r.p.RetryUntil,
		})
	}
	return out, nil
}

func (m *memRepo) Record(_ context.Context, id string, res Result) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.rows[id]
	r.rec.Status, r.rec.Attempts = res.Status, res.Attempts
	r.rec.LastError, r.rec.SentAt = res.LastError, res.SentAt
	if res.NextAttemptAt != nil {
		r.next = *res.NextAttemptAt
	}
	if res.Status != StatusPending {
		r.p.PayloadSealed, r.rec.PayloadErased = "", true
	}
	return nil
}

func (m *memRepo) Get(_ context.Context, id string) (Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok {
		return Record{}, ErrNotFound
	}
	return r.rec, nil
}

// scriptedSender records deliveries and fails as scripted.
type scriptedSender struct {
	mu       sync.Mutex
	sent     []Delivery
	failures []error // consumed one per call; nil entries succeed
}

func (s *scriptedSender) Send(_ context.Context, d Delivery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, d)
	if len(s.failures) == 0 {
		return nil
	}
	err := s.failures[0]
	s.failures = s.failures[1:]
	return err
}

func (s *scriptedSender) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sent)
}

type harness struct {
	svc   *Service
	repo  *memRepo
	email *scriptedSender
	sms   *scriptedSender
	clock *testClock
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	box, err := crypto.NewSealer([]crypto.Key{{ID: "k1", Bytes: bytes.Repeat([]byte{3}, 32)}}, "k1")
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{
		repo: newMemRepo(), email: &scriptedSender{}, sms: &scriptedSender{},
		clock: &testClock{now: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)},
	}
	h.svc, err = NewService(Deps{
		Repo: h.repo, Senders: map[Channel]Sender{Email: h.email, SMS: h.sms}, Box: box,
		Clock: h.clock, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	return h
}
