package collections

import (
	"context"
	"time"
)

// Outbox event types, one per final status.
const (
	OutboxSucceeded = "collection.succeeded"
	OutboxFailed    = "collection.failed"
	OutboxExpired   = "collection.expired"
)

// OutboxEntry announces that a collection reached a final status. It is
// written in the same transaction as the status change.
type OutboxEntry struct {
	ID           int64
	CollectionID string
	EventType    string
	CreatedAt    time.Time
}

// outboxTypes maps final statuses to their event types.
var outboxTypes = map[Status]string{
	StatusSucceeded: OutboxSucceeded,
	StatusFailed:    OutboxFailed,
	StatusExpired:   OutboxExpired,
}

// OutboxType returns the event type announcing status s, if s is final.
func OutboxType(s Status) (string, bool) {
	t, ok := outboxTypes[s]
	return t, ok
}

// OutboxHandler processes a batch of outbox entries. Returning an error
// leaves them unpublished so they are offered again.
type OutboxHandler func(ctx context.Context, entries []OutboxEntry) error
