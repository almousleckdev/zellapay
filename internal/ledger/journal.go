package ledger

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
)

// Journal is an atomic, balanced group of entries.
type Journal struct {
	ID             string
	IdempotencyKey string
	Kind           string
	Reference      string
	Entries        []Entry
}

// Validate checks the header fields and that the entries balance.
func (j Journal) Validate() error {
	if j.ID == "" || j.IdempotencyKey == "" || j.Kind == "" || j.Reference == "" {
		return fmt.Errorf("%w: missing header field", ErrInvalidJournal)
	}
	return Validate(j.Entries)
}

// Fingerprint returns a stable digest of the journal's meaning. It ignores
// ID and entry order, so a retry of the same request matches the original.
func (j Journal) Fingerprint() [sha256.Size]byte {
	h := sha256.New()
	writeField(h, j.Kind)
	writeField(h, j.Reference)
	entries := append([]Entry(nil), j.Entries...)
	sort.Slice(entries, func(a, b int) bool { return less(entries[a], entries[b]) })
	for _, e := range entries {
		writeField(h, e.Account)
		writeField(h, fmt.Sprint(int8(e.Side)))
		writeField(h, fmt.Sprint(e.Amount))
		writeField(h, string(e.Currency))
	}
	var sum [sha256.Size]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

func less(a, b Entry) bool {
	if a.Account != b.Account {
		return a.Account < b.Account
	}
	if a.Side != b.Side {
		return a.Side < b.Side
	}
	if a.Amount != b.Amount {
		return a.Amount < b.Amount
	}
	return a.Currency < b.Currency
}

type writer interface{ Write([]byte) (int, error) }

// writeField length-prefixes s so adjacent fields cannot be confused.
func writeField(w writer, s string) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(s)))
	_, _ = w.Write(n[:])
	_, _ = w.Write([]byte(s))
}

// PostResult describes the outcome of posting a journal.
type PostResult struct {
	JournalID string
	// Replayed is true when the idempotency key was already used by an
	// identical journal and nothing new was written.
	Replayed bool
}

// Poster writes journals to the ledger.
type Poster interface {
	Post(ctx context.Context, j Journal) (PostResult, error)
}

// Reader reads derived balances.
type Reader interface {
	Balance(ctx context.Context, accountID string) (int64, error)
}
