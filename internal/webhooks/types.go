// Package webhooks tells merchants about final results by signed HTTP
// callbacks. Events leave through a transactional outbox, are delivered at
// least once with exponential backoff, and can be replayed.
package webhooks

import (
	"encoding/json"
	"time"
)

// Event types merchants can subscribe to.
const (
	EventCollectionSucceeded = "collection.succeeded"
	EventCollectionFailed    = "collection.failed"
	EventCollectionExpired   = "collection.expired"
)

var knownEvents = map[string]bool{
	EventCollectionSucceeded: true,
	EventCollectionFailed:    true,
	EventCollectionExpired:   true,
}

// KnownEvent reports whether t is an event type merchants may subscribe to.
func KnownEvent(t string) bool { return knownEvents[t] }

// Endpoint is a merchant's URL that receives events.
type Endpoint struct {
	ID          string
	MerchantID  string
	URL         string
	Events      []string
	Description string
	Active      bool
	CreatedAt   time.Time
}

// SealedEndpoint is an endpoint with its signing secret encrypted.
type SealedEndpoint struct {
	Endpoint
	SecretSealed string
}

// CreatedEndpoint is returned once, when an endpoint is created. Secret is
// the only time the signing secret is available.
type CreatedEndpoint struct {
	Endpoint
	Secret string
}

// Event is something that happened to a merchant's object.
type Event struct {
	ID         string
	MerchantID string
	Type       string
	ObjectID   string
	Payload    []byte // the exact JSON body delivered to endpoints
	CreatedAt  time.Time
}

// DeliveryStatus is the state of one event to one endpoint.
type DeliveryStatus string

// Delivery states.
const (
	DeliveryPending   DeliveryStatus = "pending"
	DeliverySucceeded DeliveryStatus = "succeeded"
	DeliveryDead      DeliveryStatus = "dead" // retries exhausted
)

// Delivery tracks one event going to one endpoint.
type Delivery struct {
	ID            int64
	EventID       string
	EndpointID    string
	Status        DeliveryStatus
	Attempts      int
	NextAttemptAt *time.Time
	RetryUntil    time.Time
	LastStatus    int
	LastError     string
	CreatedAt     time.Time
	CompletedAt   *time.Time
}

// Attempt is one try to deliver, kept as the delivery log.
type Attempt struct {
	Number     int
	At         time.Time
	StatusCode int // 0 when no response was received
	Error      string
	Duration   time.Duration
}

// Work is a delivery claimed for sending, with what is needed to send it.
type Work struct {
	Delivery     Delivery
	URL          string
	SecretSealed string
	Event        Event
}

// Schedule says when a delivery is first due and how long it may be retried.
type Schedule struct {
	At    time.Time
	Until time.Time
}

// Outcome is the new state of a delivery after an attempt.
type Outcome struct {
	Status        DeliveryStatus
	NextAttemptAt *time.Time
	CompletedAt   *time.Time
}

// OutboxEntry announces that something happened to an object.
type OutboxEntry struct {
	ID        int64
	ObjectID  string
	Type      string
	CreatedAt time.Time
}

// envelope is the JSON body delivered to endpoints.
type envelope struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Data      struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}
