package pgstore_test

import (
	"os"
	"testing"

	"github.com/zellapay/zellapay/internal/platform/pgtest"
)

var testDSN string

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, func(dsn string) { testDSN = dsn }))
}
