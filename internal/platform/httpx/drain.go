package httpx

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ErrDraining is reported by a readiness check once shutdown has begun.
var ErrDraining = errors.New("httpx: draining")

// Drain records that the process has started shutting down, so readiness
// can fail while in-flight requests finish. A nil *Drain never drains.
type Drain struct {
	on      atomic.Bool
	once    sync.Once
	started chan struct{}
}

// NewDrain returns a Drain that is not yet draining.
func NewDrain() *Drain {
	return &Drain{started: make(chan struct{})}
}

// Start begins draining. It is safe to call more than once.
func (d *Drain) Start() {
	if d == nil {
		return
	}
	d.once.Do(func() {
		d.on.Store(true)
		close(d.started)
	})
}

// Draining reports whether Start has been called.
func (d *Drain) Draining() bool {
	return d != nil && d.on.Load()
}

// Started is closed when draining begins.
func (d *Drain) Started() <-chan struct{} {
	return d.started
}

// Check wraps a readiness probe. It fails with ErrDraining once draining
// has begun, and otherwise runs probe under a deadline of timeout. A nil
// probe always succeeds.
func (d *Drain) Check(
	timeout time.Duration, probe func(context.Context) error,
) func(context.Context) error {
	return func(ctx context.Context) error {
		if d.Draining() {
			return ErrDraining
		}
		if probe == nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return probe(ctx)
	}
}
