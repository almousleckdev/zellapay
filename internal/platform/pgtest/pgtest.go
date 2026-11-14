// Package pgtest starts a disposable PostgreSQL for integration tests and
// applies the embedded migrations. It must only be imported from tests.
package pgtest

import (
	"context"
	"database/sql"
	"fmt"
	"os"
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
		fmt.Fprintln(os.Stderr, "pgtest: start postgres:", err)
		return 1
	}
	defer func() { _ = c.Terminate(ctx) }()
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err == nil {
		err = migrate(dsn)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "pgtest:", err)
		return 1
	}
	set(dsn)
	return m.Run()
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
