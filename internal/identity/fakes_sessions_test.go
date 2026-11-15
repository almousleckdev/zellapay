package identity

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/platform/crypto"
)

type codeRow struct {
	userID string
	hash   [32]byte
	used   bool
}

func (m *memRepo) CreateSession(_ context.Context, s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = append(m.sessions, &s)
	return nil
}

func (m *memRepo) FindSession(_ context.Context, h [32]byte) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if s.TokenHash == h {
			return *s, nil
		}
	}
	return Session{}, ErrInvalidToken
}

func (m *memRepo) Touch(_ context.Context, id string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if s.ID == id {
			s.LastSeenAt = at
		}
	}
	return nil
}

func (m *memRepo) SetMFA(_ context.Context, id string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if s.ID == id {
			s.MFAAt = &at
		}
	}
	return nil
}

func (m *memRepo) RevokeSession(
	_ context.Context, userID, id string, at time.Time,
) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if s.ID == id && s.UserID == userID && s.RevokedAt == nil {
			s.RevokedAt = &at
			return true, nil
		}
	}
	return false, nil
}

func (m *memRepo) RevokeUserSessions(
	_ context.Context, userID, keepID string, at time.Time,
) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.revokeAll(userID, keepID, at), nil
}

// revokeAll expects the lock to be held.
func (m *memRepo) revokeAll(userID, keepID string, at time.Time) int {
	n := 0
	for _, s := range m.sessions {
		if s.UserID == userID && s.ID != keepID && s.RevokedAt == nil {
			s.RevokedAt = &at
			n++
		}
	}
	return n
}

func (m *memRepo) ListSessions(
	_ context.Context, userID string, at time.Time,
) ([]Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Session
	for _, s := range m.sessions {
		if s.UserID == userID && s.RevokedAt == nil && s.ExpiresAt.After(at) {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (m *memRepo) SaveEnrolment(_ context.Context, c MFACredential) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if old, ok := m.mfa[c.UserID]; ok && old.ConfirmedAt != nil {
		return ErrMFAAlreadyEnrolled
	}
	m.mfa[c.UserID] = c
	return nil
}

func (m *memRepo) FindMFA(_ context.Context, userID string) (MFACredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.mfa[userID]; ok {
		return c, nil
	}
	return MFACredential{}, ErrNotFound
}

func (m *memRepo) Confirm(_ context.Context, c Confirmation, fx Effects) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cred := m.mfa[c.UserID]
	cred.ConfirmedAt, cred.LastStep = &c.At, c.Step
	m.mfa[c.UserID] = cred
	m.codes = nil
	for _, h := range c.CodeHashes {
		m.codes = append(m.codes, &codeRow{userID: c.UserID, hash: h})
	}
	m.audits = append(m.audits, fx.Audit...)
	return nil
}

func (m *memRepo) AdvanceStep(_ context.Context, userID string, step int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.mfa[userID]
	if step <= c.LastStep {
		return false, nil
	}
	c.LastStep = step
	m.mfa[userID] = c
	return true, nil
}

func (m *memRepo) UseRecoveryCode(
	_ context.Context, u RecoveryUse, fx Effects,
) (bool, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ok, left := false, 0
	for _, r := range m.codes {
		if r.userID == u.UserID && r.hash == u.Hash && !r.used {
			r.used, ok = true, true
		}
	}
	if !ok {
		return false, 0, nil
	}
	for _, r := range m.codes {
		if r.userID == u.UserID && !r.used {
			left++
		}
	}
	m.audits = append(m.audits, fx.Audit...)
	return true, left, nil
}

// memThrottle locks an account after three failures, to keep tests short.
type memThrottle struct {
	fails map[string]int
	down  bool
}

func (t *memThrottle) key(a Attempt) string { return string(a.Realm) + ":" + a.Account }

func (t *memThrottle) Wait(_ context.Context, a Attempt) (time.Duration, error) {
	if t.down {
		return 0, context.DeadlineExceeded
	}
	if t.fails[t.key(a)] >= 3 {
		return time.Minute, nil
	}
	return 0, nil
}

func (t *memThrottle) Failed(_ context.Context, a Attempt) error {
	t.fails[t.key(a)]++
	return nil
}

func (t *memThrottle) Succeeded(_ context.Context, a Attempt) error {
	delete(t.fails, t.key(a))
	return nil
}

func testBox(t *testing.T) *crypto.Sealer {
	t.Helper()
	box, err := crypto.NewSealer(
		[]crypto.Key{{ID: "k1", Bytes: bytes.Repeat([]byte{8}, 32)}}, "k1")
	if err != nil {
		t.Fatal(err)
	}
	return box
}
