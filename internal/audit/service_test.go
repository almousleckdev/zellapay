package audit

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return at }

// memRepo is an in-memory log with the same chain rules as the real one.
type memRepo struct {
	mu    sync.Mutex
	rows  []Record
	head  Head
	state VerifyState
}

func (m *memRepo) Append(_ context.Context, p Pending) (Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := Record{
		Entry: p.Entry, Seq: m.head.Seq + 1, At: p.At,
		ChangeJSON: p.ChangeJSON, PrevHash: m.head.Hash,
	}
	r.Hash = Link(r.PrevHash, r.Seq, p)
	m.rows = append(m.rows, r)
	m.head = Head{Seq: r.Seq, Hash: r.Hash}
	return r, nil
}

func (m *memRepo) Head(context.Context) (Head, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.head, nil
}

func (m *memRepo) After(_ context.Context, seq int64, limit int) ([]Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Record
	for _, r := range m.rows {
		if r.Seq > seq && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memRepo) List(context.Context, Filter) ([]Record, error) { return m.rows, nil }

func (m *memRepo) Verification(context.Context) (VerifyState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state, nil
}

func (m *memRepo) SaveVerification(_ context.Context, s VerifyState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = s
	return nil
}

func newService() (*Service, *memRepo) {
	repo := &memRepo{}
	return NewService(repo, fixedClock{}), repo
}

func (s *Service) must(t *testing.T, action string) Record {
	t.Helper()
	e := valid()
	e.Action = action
	r, err := s.Record(context.Background(), e)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestEntriesFormAChain(t *testing.T) {
	s, _ := newService()
	first, second := s.must(t, "a.one"), s.must(t, "a.two")
	chained := first.Seq == 1 && first.PrevHash == Genesis &&
		second.Seq == 2 && second.PrevHash == first.Hash
	if !chained {
		t.Fatalf("not chained: %+v %+v", first, second)
	}
	if Recompute(first) != first.Hash || Recompute(second) != second.Hash {
		t.Fatal("a stored entry must recompute to its own hash")
	}
}

func TestAnInvalidEntryIsNeverStored(t *testing.T) {
	s, repo := newService()
	e := valid()
	e.Reason = ""
	if _, err := s.Record(context.Background(), e); err == nil || len(repo.rows) != 0 {
		t.Fatalf("err=%v rows=%d", err, len(repo.rows))
	}
}
