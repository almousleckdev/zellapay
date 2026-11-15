// Package pgtest starts a disposable PostgreSQL for integration tests and
// applies the embedded migrations. It must only be imported from tests.
package pgtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/zellapay/zellapay/db/migrations"
)

const startupTimeout = 60 * time.Second

// Main starts PostgreSQL, migrates it, hands the connection string to
// set, runs the tests and cleans up. Call it from TestMain:
//
//	os.Exit(pgtest.Main(m, func(dsn string) { testDSN = dsn }))
func Main(m *testing.M, set func(dsn string)) int {
	dsn, stop, err := Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, "pgtest:", err)
		return 1
	}
	defer stop()
	set(dsn)
	return m.Run()
}

// Start launches a migrated PostgreSQL and returns its connection string and
// a function that removes it. Use it when a test package needs more than one
// container; otherwise prefer Main.
func Start() (dsn string, stop func(), err error) {
	ctx := context.Background()
	c, err := tcpostgres.Run(ctx, "postgres:17",
		tcpostgres.WithDatabase("zellapay"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(startupTimeout)),
	)
	if err != nil {
		return "", nil, fmt.Errorf("start postgres: %w", err)
	}
	stop = func() { _ = c.Terminate(ctx) }
	if dsn, err = c.ConnectionString(ctx, "sslmode=disable"); err == nil {
		err = migrate(dsn)
	}
	if err != nil {
		stop()
		return "", nil, err
	}
	return dsn, stop, nil
}

func migrate(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, ".")
}

// NewPool opens a pool on dsn and closes it when the test ends.
func NewPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 32
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Fresh creates a new, migrated database inside the same server and returns a
// pool to it. Use it for tests that damage data on purpose, so they cannot
// disturb other tests. The database is dropped when the test ends.
func Fresh(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	admin := NewPool(t, dsn)
	name := "fresh_" + strings.ReplaceAll(uuidLike(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	fresh := u.String()
	if err := migrate(fresh); err != nil {
		t.Fatal(err)
	}
	pool := NewPool(t, fresh)
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})
	return pool
}

func uuidLike() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
