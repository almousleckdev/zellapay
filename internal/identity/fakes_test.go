package identity

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/notify"
)

var epoch = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time          { return c.now }
func (c *testClock) advance(d time.Duration) { c.now = c.now.Add(d) }

// memRepo is an in-memory Repository with the same single-use rules.
type memRepo struct {
	mu     sync.Mutex
	users  map[string]User
	invs   []*invRow
	tokens []*tokRow
	audits []audit.Pending

	sessions []*Session
	mfa      map[string]MFACredential
	codes    []*codeRow
}

type invRow struct {
	Invitation
	used, revoked bool
}

type tokRow struct {
	EmailToken
	used bool
}

func newMemRepo() *memRepo {
	return &memRepo{users: map[string]User{}, mfa: map[string]MFACredential{}}
}

func (m *memRepo) CreateInvitation(_ context.Context, inv Invitation, fx Effects) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.invs {
		if r.MerchantID == inv.MerchantID && r.Email == inv.Email && !r.used {
			r.revoked = true
		}
	}
	m.invs = append(m.invs, &invRow{Invitation: inv})
	m.audits = append(m.audits, fx.Audit...)
	return nil
}

func (m *memRepo) FindInvitation(_ context.Context, h [32]byte) (Invitation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.invs {
		if r.TokenHash == h && !r.used && !r.revoked {
			return r.Invitation, nil
		}
	}
	return Invitation{}, ErrInvalidToken
}

func (m *memRepo) AcceptInvitation(_ context.Context, a Acceptance, fx Effects) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var row *invRow
	for _, r := range m.invs {
		if r.TokenHash == a.TokenHash && !r.used && !r.revoked && r.ExpiresAt.After(a.At) {
			row = r
		}
	}
	if row == nil {
		return ErrInvalidToken
	}
	for _, u := range m.users {
		if u.Email == a.User.Email {
			return ErrEmailTaken
		}
	}
	row.used = true
	m.users[a.User.ID] = a.User
	m.audits = append(m.audits, fx.Audit...)
	return nil
}

func (m *memRepo) FindUserByEmail(_ context.Context, email string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return User{}, ErrNotFound
}

func (m *memRepo) FindUser(_ context.Context, id string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return User{}, ErrNotFound
}

func (m *memRepo) Rehash(_ context.Context, id, oldHash, newHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u := m.users[id]; u.PasswordHash == oldHash {
		u.PasswordHash = newHash
		m.users[id] = u
	}
	return nil
}

func (m *memRepo) CreateEmailToken(_ context.Context, t EmailToken, fx Effects) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	kept := make([]*tokRow, 0, len(m.tokens)+1)
	for _, r := range m.tokens {
		if r.used || r.UserID != t.UserID || r.Purpose != t.Purpose {
			kept = append(kept, r)
		}
	}
	kept = append(kept, &tokRow{EmailToken: t})
	m.tokens = kept
	m.audits = append(m.audits, fx.Audit...)
	return nil
}

func (m *memRepo) FindEmailToken(_ context.Context, p Purpose, h [32]byte) (EmailToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.tokens {
		if r.TokenHash == h && r.Purpose == p && !r.used {
			return r.EmailToken, nil
		}
	}
	return EmailToken{}, ErrInvalidToken
}

func (m *memRepo) UseEmailToken(_ context.Context, use TokenUse, fx Effects) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.tokens {
		if r.TokenHash == use.TokenHash && r.Purpose == use.Purpose &&
			!r.used && r.ExpiresAt.After(use.At) {
			r.used = true
			u := m.users[use.UserID]
			if use.NewPasswordHash != "" {
				u.PasswordHash = use.NewPasswordHash
				m.revokeAll(use.UserID, "", use.At)
			}
			if u.EmailVerifiedAt == nil {
				at := use.At
				u.EmailVerifiedAt = &at
			}
			m.users[use.UserID] = u
			m.audits = append(m.audits, fx.Audit...)
			return nil
		}
	}
	return ErrInvalidToken
}

// sentMail records messages instead of queueing them.
type sentMail struct {
	mu   sync.Mutex
	sent []notify.Message
}

func (s *sentMail) Prepare(m notify.Message) (notify.Prepared, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, m)
	return notify.Prepared{ID: "ntf_test"}, nil
}

func (s *sentMail) last(t *testing.T) notify.Message {
	t.Helper()
	if len(s.sent) == 0 {
		t.Fatal("no email was prepared")
	}
	return s.sent[len(s.sent)-1]
}

// realAuditor validates entries exactly as production does.
type realAuditor struct{ svc *audit.Service }

func (a realAuditor) Prepare(e audit.Entry) (audit.Pending, error) { return a.svc.Prepare(e) }

type directory map[string]string

func (d directory) BusinessName(_ context.Context, id string) (string, error) {
	if n, ok := d[id]; ok {
		return n, nil
	}
	return "", ErrNotFound
}

type rig struct {
	svc      *Service
	sessions *Sessions
	mfa      *MFA
	repo     *memRepo
	mail     *sentMail
	clock    *testClock
	throttle *memThrottle
}

func newRig(t *testing.T) *rig {
	t.Helper()
	clk := &testClock{now: epoch}
	repo, mail := newMemRepo(), &sentMail{}
	d := Deps{
		Repo: repo, Hasher: testHasher(t, cheap(), 1, testPepper), Mailer: mail,
		Auditor:   realAuditor{audit.NewService(nil, clk)},
		Directory: directory{"mer_1": "Boutique Soleil"}, Clock: clk,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	svc, err := NewService(d, DefaultConfig("https://merchant.test/"))
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{svc: svc, repo: repo, mail: mail, clock: clk,
		throttle: &memThrottle{fails: map[string]int{}}}
	r.wireSessions(t, d)
	return r
}
