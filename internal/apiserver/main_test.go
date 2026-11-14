package apiserver_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/zellapay/zellapay/internal/platform/pgtest"
	"github.com/zellapay/zellapay/internal/platform/redistest"
)

var testDSN, testRedis string

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	dsn, stopPG, err := pgtest.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer stopPG()
	url, stopRedis, err := redistest.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer stopRedis()
	testDSN, testRedis = dsn, url
	return m.Run()
}
