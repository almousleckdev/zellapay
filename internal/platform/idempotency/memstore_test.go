package idempotency

import (
	"bytes"
	"context"
	"sync"
	"time"
)

type memRecord struct {
	fingerprint []byte
	done        bool
	resp        Response
}

// memStore is an in-memory Store with the same contract as the real one.
type memStore struct {
	mu        sync.Mutex
	recs      map[string]*memRecord
	completes int
	releases  int
}

func newMemStore() *memStore { return &memStore{recs: map[string]*memRecord{}} }

func (m *memStore) Begin(_ context.Context, r Request, _ time.Time) (Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := r.Scope + "/" + r.Key
	rec, ok := m.recs[id]
	switch {
	case !ok:
		m.recs[id] = &memRecord{fingerprint: r.Fingerprint}
		return Decision{Kind: Started}, nil
	case !bytes.Equal(rec.fingerprint, r.Fingerprint):
		return Decision{Kind: Conflict}, nil
	case rec.done:
		return Decision{Kind: Replay, Response: rec.resp}, nil
	}
	return Decision{Kind: InFlight}, nil
}

func (m *memStore) Complete(_ context.Context, r Request, resp Response, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := m.recs[r.Scope+"/"+r.Key]
	rec.done, rec.resp = true, resp
	m.completes++
	return nil
}

func (m *memStore) Release(_ context.Context, r Request) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.recs, r.Scope+"/"+r.Key)
	m.releases++
	return nil
}
