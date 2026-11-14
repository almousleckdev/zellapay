package notify

import (
	"context"
	"time"
)

// Sender delivers one rendered message over one channel.
type Sender interface {
	Send(ctx context.Context, d Delivery) error
}

// SecretBox seals and opens message payloads.
type SecretBox interface {
	Seal(plaintext, aad []byte) (string, error)
	Open(token string, aad []byte) ([]byte, error)
}

// Repository stores the outbox.
type Repository interface {
	// Insert stores p. It reports false when a message with the same dedupe
	// key already exists, in which case nothing is stored.
	Insert(ctx context.Context, p Prepared) (bool, error)
	// ClaimDue returns messages due at now and pushes their next attempt
	// forward by lease, so concurrent workers never send the same one.
	ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]Queued, error)
	// Record stores the outcome of an attempt. A message that is sent or dead
	// has its payload erased.
	Record(ctx context.Context, id string, r Result) error
	Get(ctx context.Context, id string) (Record, error)
}
