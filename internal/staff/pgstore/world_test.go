package pgstore_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zellapay/zellapay/internal/audit"
	auditstore "github.com/zellapay/zellapay/internal/audit/pgstore"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/pgtest"
	"github.com/zellapay/zellapay/internal/staff"
	"github.com/zellapay/zellapay/internal/staff/pgstore"
)

const phrase = "tournesol bleu sur la table"

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type openThrottle struct{}

func (openThrottle) Wait(context.Context, identity.Attempt) (time.Duration, error) { return 0, nil }
func (openThrottle) Failed(context.Context, identity.Attempt) error                { return nil }
func (openThrottle) Succeeded(context.Context, identity.Attempt) error             { return nil }

type world struct {
	svc   *staff.Service
	pool  *pgxpool.Pool
	clock *clock
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func newWorld(t *testing.T) *world {
	t.Helper()
	pool := pgtest.Fresh(t, testDSN)
	clk := &clock{now: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)}
	wa := must(webauthn.New(&webauthn.Config{
		RPDisplayName: "ZellaPay Admin", RPID: "localhost",
		RPOrigins: []string{"http://localhost:3002"},
	}))
	hasher := must(identity.NewHasher(
		identity.Params{Memory: 8, Passes: 1, Lanes: 1, Version: 1},
		map[int][]byte{1: bytes.Repeat([]byte("p"), 32)}))
	svc := must(staff.NewService(staff.Deps{
		Repo:     pgstore.New(pool, auditstore.New(pool)),
		Hasher:   hasher,
		Auditor:  audit.NewService(auditstore.New(pool), clk),
		Throttle: openThrottle{}, WebAuthn: wa, Clock: clk,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, staff.DefaultConfig()))
	return &world{svc: svc, pool: pool, clock: clk}
}

func (w *world) superAdmin(t *testing.T) staff.User {
	t.Helper()
	u, err := w.svc.Create(context.Background(), staff.CreateInput{
		Email: "Boss@Example.com", Name: "The Boss", Role: staff.RoleSuperAdmin, Password: phrase,
		By: audit.Actor{Kind: audit.ActorSystem, ID: "zellactl"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func (w *world) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
