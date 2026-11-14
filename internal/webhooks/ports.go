package webhooks

import (
	"context"
	"encoding/json"
	"time"
)

// SendRequest is one HTTP delivery.
type SendRequest struct {
	URL     string
	Body    []byte
	Headers map[string]string
}

// Sender performs an HTTP POST. It returns the response status, or a
// *SendError when no response arrived.
type Sender interface {
	Send(ctx context.Context, req SendRequest) (status int, err error)
}

// ClaimHandler processes claimed outbox entries.
type ClaimHandler func(ctx context.Context, entries []OutboxEntry) error

// Feed supplies the outbox: things that happened and must be announced.
type Feed interface {
	// Claim offers up to limit unpublished entries to handle and marks them
	// published only if handle returns nil.
	Claim(ctx context.Context, now time.Time, limit int, handle ClaimHandler) (int, error)
}

// PayloadSource describes the object an outbox entry is about.
type PayloadSource interface {
	// Object returns the owning merchant and the JSON representation of the
	// object, exactly as the merchant API presents it.
	Object(ctx context.Context, entry OutboxEntry) (string, json.RawMessage, error)
}

// SecretBox encrypts and decrypts endpoint signing secrets.
type SecretBox interface {
	Seal(plaintext, aad []byte) (string, error)
	Open(token string, aad []byte) ([]byte, error)
}

// Repository stores endpoints, events and deliveries.
type Repository interface {
	// CreateEndpoint stores e unless the merchant already has max active
	// endpoints, in which case it returns ErrTooManyEndpoints.
	CreateEndpoint(ctx context.Context, e SealedEndpoint, max int) error
	ListEndpoints(ctx context.Context, merchantID string) ([]Endpoint, error)
	// EndpointsFor returns the merchant's active endpoints that subscribe to
	// eventType, with their sealed secrets.
	EndpointsFor(ctx context.Context, merchantID, eventType string) ([]Endpoint, error)

	// SaveEvent stores ev. An event for the same type and object already
	// stored is returned unchanged, which makes publishing idempotent.
	SaveEvent(ctx context.Context, ev Event) (Event, error)
	GetEvent(ctx context.Context, merchantID, eventID string) (Event, error)

	// Enqueue schedules delivery of an event to an endpoint, once.
	Enqueue(ctx context.Context, eventID, endpointID string, s Schedule) error
	// ClaimDue returns deliveries due at now and pushes their next attempt
	// forward by lease, so concurrent workers never send the same one.
	ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]Work, error)
	// Record stores an attempt and the delivery's new state in one step.
	Record(ctx context.Context, deliveryID int64, a Attempt, o Outcome) error
	// Rearm makes all of an event's deliveries pending again for replay.
	Rearm(ctx context.Context, eventID string, now, until time.Time) (int, error)

	Deliveries(ctx context.Context, eventID string) ([]Delivery, error)
	Attempts(ctx context.Context, deliveryID int64) ([]Attempt, error)
}
