// Package redistest starts a disposable Redis for integration tests. It must
// only be imported from tests.
package redistest

import (
	"context"
	"fmt"
	"os"
	"testing"

	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// Main starts Redis, hands its URL to set, runs the tests and cleans up.
// Call it from TestMain:
//
//	os.Exit(redistest.Main(m, func(url string) { redisURL = url }))
func Main(m *testing.M, set func(url string)) int {
	url, stop, err := Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, "redistest:", err)
		return 1
	}
	defer stop()
	set(url)
	return m.Run()
}

// Start launches Redis and returns its URL and a function that removes it.
func Start() (url string, stop func(), err error) {
	ctx := context.Background()
	c, err := tcredis.Run(ctx, "redis:7")
	if err != nil {
		return "", nil, fmt.Errorf("start redis: %w", err)
	}
	stop = func() { _ = c.Terminate(ctx) }
	if url, err = c.ConnectionString(ctx); err != nil {
		stop()
		return "", nil, err
	}
	return url, stop, nil
}
