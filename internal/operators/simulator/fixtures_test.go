package simulator

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/operators"
)

var (
	epoch  = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	secret = []byte("sim-test-secret")
)

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time          { return c.now }
func (c *testClock) advance(d time.Duration) { c.now = c.now.Add(d) }

func newSim(t *testing.T) (*Simulator, *testClock) {
	t.Helper()
	clk := &testClock{now: epoch}
	return New(Config{Delay: 5 * time.Second, Secret: secret}, clk), clk
}

// phone builds a number whose last two digits select a scenario.
func phone(suffix string) string { return "+241070000" + suffix }

func collect(t *testing.T, s *Simulator, suffix string) (operators.Ref, error) {
	t.Helper()
	return s.InitiateCollection(context.Background(), operators.CollectionRequest{
		Reference: "ref-" + suffix, Phone: phone(suffix), Amount: 10000, Currency: "XAF",
	})
}

func statusOf(t *testing.T, s *Simulator, ref operators.Ref) operators.State {
	t.Helper()
	st, err := s.Status(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	return st.State
}

func callbackRequest(cb Callback) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost, "/callbacks/simulator", bytes.NewReader(cb.Body))
	req.Header.Set(SignatureHeader, cb.Signature)
	return req
}
