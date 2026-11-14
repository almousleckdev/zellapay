package pgstore_test

import (
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zellapay/zellapay/internal/platform/pgtest"
)

var testDSN string

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, func(dsn string) { testDSN = dsn }))
}

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return pgtest.NewPool(t, testDSN)
}
