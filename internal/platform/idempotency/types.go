// Package idempotency makes unsafe HTTP requests safe to retry. A request
// carrying an Idempotency-Key executes at most once per merchant; retries
// receive the original response.
package idempotency

import (
	"context"
	"errors"
	"time"
)

// Kind is the outcome of trying to begin a request.
type Kind int8

// Possible outcomes of Store.Begin.
const (
	// Started means the caller owns the key and must execute the request.
	Started Kind = iota + 1
	// Replay means the request already completed; use Decision.Response.
	Replay
	// InFlight means another request with the same key is still running.
	InFlight
	// Conflict means the key was used earlier with a different request.
	Conflict
)

// ErrLockLost means the in-progress record was taken over or removed
// before the request finished.
var ErrLockLost = errors.New("idempotency: lock lost")

// Request identifies one idempotent request.
type Request struct {
	Scope       string // the tenant, normally the merchant ID
	Key         string // the client's Idempotency-Key
	Fingerprint []byte // digest of method, path, query and body
}

// Response is the stored result of a completed request.
type Response struct {
	Status      int
	ContentType string
	Body        []byte
}

// Decision tells the middleware what to do next.
type Decision struct {
	Kind     Kind
	Response Response // set when Kind is Replay
}

// Store persists idempotency records.
type Store interface {
	// Begin claims the key or reports why it cannot be claimed.
	Begin(ctx context.Context, req Request, now time.Time) (Decision, error)
	// Complete stores the final response for a claimed key.
	Complete(ctx context.Context, req Request, resp Response, now time.Time) error
	// Release abandons a claimed key so the client can retry.
	Release(ctx context.Context, req Request) error
}
