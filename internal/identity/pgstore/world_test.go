package pgstore_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zellapay/zellapay/internal/audit"
	auditstore "github.com/zellapay/zellapay/internal/audit/pgstore"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/identity/pgstore"
	"github.com/zellapay/zellapay/internal/notify"
	notifystore "github.com/zellapay/zellapay/internal/notify/pgstore"
	"github.com/zellapay/zellapay/internal/platform/crypto"
	"github.com/zellapay/zellapay/internal/platform/pgtest"
)

const phrase = "tournesol bleu sur la table"

type fixedClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fixedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fixedClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type directory struct{}

func (directory) BusinessName(context.Context, string) (string, error) {
	return "Boutique Soleil", nil
}

type noThrottle struct{}

func (noThrottle) Wait(context.Context, identity.Attempt) (time.Duration, error) { return 0, nil }
func (noThrottle) Failed(context.Context, identity.Attempt) error                { return nil }
func (noThrottle) Succeeded(context.Context, identity.Attempt) error             { return nil }

type world struct {
	sessions *identity.Sessions
	mfa      *identity.MFA
	svc      *identity.Service
	pool     *pgxpool.Pool
	mail     *notifystore.Store
	audit    *audit.Service
	clock    *fixedClock
}

// must returns v, or fails the test by panicking when setup went wrong.
func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// newWorld wires identity to real notify and audit stores on a fresh database.
func newWorld(t *testing.T) *world {
	t.Helper()
	pool := pgtest.Fresh(t, testDSN)
	clk := &fixedClock{now: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)}
	box := must(crypto.NewSealer(
		[]crypto.Key{{ID: "k1", Bytes: bytes.Repeat([]byte{5}, 32)}}, "k1"))
	mailStore, auditStore := notifystore.New(pool), auditstore.New(pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mailer := must(notify.NewService(notify.Deps{
		Repo: mailStore, Senders: map[notify.Channel]notify.Sender{notify.Email: nil},
		Box: box, Clock: clk, Log: log,
	}, notify.DefaultConfig()))
	hasher := must(identity.NewHasher(
		identity.Params{Memory: 8, Passes: 1, Lanes: 1, Version: 1},
		map[int][]byte{1: bytes.Repeat([]byte("p"), 32)}))
	auditSvc := audit.NewService(auditStore, clk)
	repo := pgstore.New(pool, mailStore, auditStore)
	svc := must(identity.NewService(identity.Deps{
		Repo: repo, Hasher: hasher, Mailer: mailer, Auditor: auditSvc,
		Directory: directory{}, Clock: clk, Log: log,
	}, identity.DefaultConfig("https://merchant.test")))
	mfa := must(identity.NewMFA(identity.MFADeps{
		Repo: repo, Users: repo, Box: box, Auditor: auditSvc, Clock: clk, Issuer: "ZellaPay",
	}))
	sessions := must(identity.NewSessions(identity.SessionDeps{
		Repo: repo, Users: repo, Accounts: svc, Factor: mfa, Throttle: noThrottle{},
		Clock: clk, Log: log,
	}, identity.MerchantSessionConfig()))
	w := &world{
		sessions: sessions, mfa: mfa, svc: svc, pool: pool,
		mail: mailStore, audit: auditSvc, clock: clk,
	}
	w.exec(t, `INSERT INTO merchants (id, name, default_currency, created_at, updated_at)
		VALUES ('mer_1', 'Boutique Soleil', 'XAF', now(), now())`)
	return w
}

func (w *world) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := w.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func (w *world) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (w *world) invite(t *testing.T, email string) identity.Invitation {
	t.Helper()
	inv, err := w.svc.Invite(context.Background(), identity.InviteInput{
		MerchantID: "mer_1", Email: email, Role: identity.RoleOwner,
		InvitedBy: audit.Actor{Kind: audit.ActorSystem, ID: "bootstrap"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return inv
}

// tokenFor recovers the emailed token. Real tokens are never stored, so the
// test makes its own and swaps in its hash.
func (w *world) tokenFor(t *testing.T, table, id string) string {
	t.Helper()
	token := "test-token-" + strings.Repeat("x", 32) + id
	w.exec(t, "UPDATE "+table+" SET token_hash = sha256($1::bytea) WHERE id = $2", token, id)
	return token
}
