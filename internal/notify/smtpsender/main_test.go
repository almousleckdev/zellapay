package smtpsender

import (
	"fmt"
	"os"
	"testing"

	"github.com/zellapay/zellapay/internal/platform/maildevtest"
)

var maildev *maildevtest.Server

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	s, err := maildevtest.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer s.Stop()
	maildev = s
	return m.Run()
}
