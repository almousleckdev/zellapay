package ledger

import (
	"context"
	"fmt"
	"sort"

	"github.com/zellapay/zellapay/internal/platform/digest"
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
func (j Journal) Fingerprint() [digest.Size]byte {
	entries := append([]Entry(nil), j.Entries...)
	sort.Slice(entries, func(a, b int) bool { return less(entries[a], entries[b]) })
	fields := make([]string, 0, 2+4*len(entries))
	fields = append(fields, j.Kind, j.Reference)
	for _, e := range entries {
		fields = append(fields, e.Account, fmt.Sprint(int8(e.Side)),
			fmt.Sprint(e.Amount), string(e.Currency))
	}
	return digest.Sum(fields...)
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
