// Package simulator is a scripted mobile money operator for development,
// the sandbox and tests. Behaviour is chosen by the customer's phone number.
package simulator

import (
	"context"
	"sync"
	"time"

	"github.com/zellapay/zellapay/internal/operators"
	"github.com/zellapay/zellapay/internal/platform/clock"
	"github.com/zellapay/zellapay/internal/platform/ids"
)

// Code is the simulator's operator code.
const Code = "simulator"

const defaultDelay = 2 * time.Second

// spec is what a caller asked for. Two requests are the same request
// exactly when their specs are equal.
type spec struct {
	kind      string // "collection" or "payout"
	reference string
	phone     string
	currency  string
	amount    int64
}

// txn is one transaction the simulated operator knows about.
type txn struct {
	spec
	opRef    string
	scenario scenario
	created  time.Time
	sent     int // callbacks already emitted
}

// Simulator implements operators.Adapter in memory.
type Simulator struct {
	cfg   Config
	clock clock.Clock
	mu    sync.Mutex
	txns  map[string]*txn // keyed by our reference
}

var _ operators.Adapter = (*Simulator)(nil)

// New returns a Simulator. A zero Delay defaults to two seconds.
func New(cfg Config, c clock.Clock) *Simulator {
	if cfg.Delay <= 0 {
		cfg.Delay = defaultDelay
	}
	return &Simulator{cfg: cfg, clock: c, txns: map[string]*txn{}}
}

// Code identifies the adapter.
func (s *Simulator) Code() string { return Code }

// InitiateCollection starts a collection.
func (s *Simulator) InitiateCollection(
	_ context.Context, r operators.CollectionRequest,
) (operators.Ref, error) {
	return s.initiate(spec{
		kind: "collection", reference: r.Reference, phone: r.Phone,
		currency: r.Currency, amount: r.Amount,
	})
}

// InitiatePayout starts a payout.
func (s *Simulator) InitiatePayout(
	_ context.Context, r operators.PayoutRequest,
) (operators.Ref, error) {
	return s.initiate(spec{
		kind: "payout", reference: r.Reference, phone: r.Phone,
		currency: r.Currency, amount: r.Amount,
	})
}

// initiate records the transaction. Repeating an identical request returns
// the same reference, as real operators do; a different request under the
// same reference is rejected.
func (s *Simulator) initiate(sp spec) (operators.Ref, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if prev, ok := s.txns[sp.reference]; ok {
		if prev.spec != sp {
			return operators.Ref{}, operators.ErrDuplicateReference
		}
		return prev.ref(), s.initiationError(prev)
	}
	t := &txn{
		spec: sp, opRef: ids.New("sim_"),
		scenario: scenarioFor(sp.phone), created: s.clock.Now(),
	}
	s.txns[sp.reference] = t
	return t.ref(), s.initiationError(t)
}

func (t *txn) ref() operators.Ref {
	return operators.Ref{OperatorRef: t.opRef, OurRef: t.reference}
}

// initiationError reports a timeout for the scripted timeout scenario even
// though the operator processed the request, as a real network failure can.
func (s *Simulator) initiationError(t *txn) error {
	switch t.scenario {
	case scenTimeoutThenOK:
		return operators.ErrUnknownOutcome
	case scenRejected:
		return operators.ErrRejected
	}
	return nil
}

// Health reports the simulator as always available.
func (s *Simulator) Health(context.Context) operators.Health {
	return operators.Health{OK: true, CheckedAt: s.clock.Now(), Detail: "simulator"}
}
