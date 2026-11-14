// Command migrate applies database migrations. It uses its own database
// role, separate from the services, because only it may change the schema.
package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"

	"github.com/zellapay/zellapay/db/migrations"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 || (args[0] != "up" && args[0] != "status") {
		return fmt.Errorf("usage: migrate up|status")
	}
	dsn := os.Getenv("ZP_MIGRATE_DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("ZP_MIGRATE_DATABASE_URL: required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if args[0] == "status" {
		return goose.Status(db, ".")
	}
	return goose.Up(db, ".")
}
