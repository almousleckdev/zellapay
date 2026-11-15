package audit

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/zellapay/zellapay/internal/platform/digest"
)

// Genesis is the hash that precedes the first entry.
var Genesis Hash

// Link returns the hash of an entry that follows prev at position seq. It
// covers every field, so changing any stored value changes the hash. Fields
// are length-prefixed, so values can never be shifted from one field into the
// next to produce the same hash.
func Link(prev Hash, seq int64, p Pending) Hash {
	return Hash(digest.Sum(
		string(prev[:]),
		strconv.FormatInt(seq, 10),
		p.At.UTC().Format(time.RFC3339Nano),
		string(p.Actor.Kind), p.Actor.ID,
		p.Action,
		p.Target.Kind, p.Target.ID,
		p.Reason,
		p.ChangeJSON,
		p.Source.IP, p.Source.UserAgent, p.Source.RequestID,
	))
}

// canonicalChange renders a change map as JSON with sorted keys, so the same
// change always produces the same bytes.
func canonicalChange(change map[string]string) (string, error) {
	if len(change) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(change)
	return string(b), err
}

// pendingOf rebuilds the hashed form of a stored record, to re-check it.
func pendingOf(r Record) Pending {
	return Pending{Entry: r.Entry, At: r.At, ChangeJSON: r.ChangeJSON}
}

// Recompute returns the hash a stored record should have.
func Recompute(r Record) Hash { return Link(r.PrevHash, r.Seq, pendingOf(r)) }

// StampTime reduces a time to the precision the database stores, so the value
// that is hashed is exactly the value that is read back later.
func StampTime(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) }
