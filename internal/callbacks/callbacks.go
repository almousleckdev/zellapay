// Package callbacks receives notifications from mobile money operators and
// hands each one to the module that owns the referenced transaction. A
// callback is verified and parsed once, here, by the operator's adapter.
package callbacks

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/zellapay/zellapay/internal/operators"
)

// Errors a Sink or the dispatcher can return.
var (
	// ErrUnknownOperator means no adapter is registered under the code.
	ErrUnknownOperator = errors.New("callbacks: unknown operator")
	// ErrUnknownReference means no module recognises the reference.
	ErrUnknownReference = errors.New("callbacks: unknown reference")
)

// Sink is a module that owns transactions the operator may call back about.
type Sink interface {
	// Owns reports whether reference was issued by this module.
	Owns(reference string) bool
	// HandleEvent processes a verified callback. It returns
	// ErrUnknownReference if it has no such transaction.
	HandleEvent(ctx context.Context, operatorCode string, ev operators.CallbackEvent) error
}

// Adapters finds operator adapters by code.
type Adapters interface {
	Adapter(code string) (operators.Adapter, bool)
}

// Dispatcher verifies callbacks and routes them to sinks.
type Dispatcher struct {
	adapters Adapters
	sinks    []Sink
}

// New returns a Dispatcher over the given sinks.
func New(adapters Adapters, sinks ...Sink) *Dispatcher {
	return &Dispatcher{adapters: adapters, sinks: sinks}
}

// Handle verifies the request with the named operator's adapter and passes
// the event to the sink that owns its reference.
func (d *Dispatcher) Handle(ctx context.Context, operatorCode string, r *http.Request) error {
	a, ok := d.adapters.Adapter(operatorCode)
	if !ok {
		return ErrUnknownOperator
	}
	ev, err := a.ParseCallback(ctx, r)
	if err != nil {
		return err
	}
	for _, s := range d.sinks {
		if s.Owns(ev.Ref.OurRef) {
			return s.HandleEvent(ctx, operatorCode, ev)
		}
	}
	return ErrUnknownReference
}

// Handler serves POST /callbacks/{operator}. It is called by operators, not
// merchants, and is protected by the adapter's own signature check.
//
// A reference nobody knows is acknowledged: the operator cannot fix it, and a
// retry would only add noise.
func (d *Dispatcher) Handler(log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := d.Handle(r.Context(), r.PathValue("operator"), r)
		switch {
		case err == nil:
			w.WriteHeader(http.StatusNoContent)
		case errors.Is(err, ErrUnknownReference):
			log.Warn("callback for an unknown reference", "operator", r.PathValue("operator"))
			w.WriteHeader(http.StatusNoContent)
		case errors.Is(err, ErrUnknownOperator):
			http.NotFound(w, r)
		case errors.Is(err, operators.ErrInvalidCallback):
			w.WriteHeader(http.StatusBadRequest)
		default:
			log.Error("callback failed", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
