package audit

import (
	"context"
	"time"
)

// Filter selects entries to read, newest first.
type Filter struct {
	ActorKind  ActorKind
	ActorID    string
	TargetKind string
	TargetID   string
	Action     string
	BeforeSeq  int64 // return entries older than this; 0 means from the newest
	Limit      int   // 1 to 200; zero means 50
}

// VerifyState remembers how far the chain has been checked, so each pass only
// examines new entries.
type VerifyState struct {
	At           time.Time // when this state was saved
	VerifiedSeq  int64
	VerifiedHash Hash
	BrokenSeq    int64 // zero while the chain is intact
	BrokenReason string
}

// Repository stores the log.
type Repository interface {
	// Append chains p after the current head and stores it, atomically.
	Append(ctx context.Context, p Pending) (Record, error)
	Head(ctx context.Context) (Head, error)
	// After returns entries with a higher sequence number, oldest first.
	After(ctx context.Context, seq int64, limit int) ([]Record, error)
	List(ctx context.Context, f Filter) ([]Record, error)
	Verification(ctx context.Context) (VerifyState, error)
	SaveVerification(ctx context.Context, s VerifyState) error
}
