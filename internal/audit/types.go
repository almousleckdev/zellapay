// Package audit keeps a tamper-evident record of every sensitive action. The
// log is append-only, and each entry carries a hash of the one before it, so
// editing or deleting any past entry breaks every hash that follows. A
// scheduled check re-computes the chain and raises the alarm on a break.
package audit

import "time"

// ActorKind says who acted.
type ActorKind string

// Kinds of actor.
const (
	ActorMerchantUser ActorKind = "merchant_user"
	ActorStaff        ActorKind = "staff"
	ActorAPIKey       ActorKind = "api_key"
	ActorSystem       ActorKind = "system"
)

// Actor is who did something.
type Actor struct {
	Kind ActorKind
	ID   string
}

// Target is what was acted on.
type Target struct {
	Kind string // for example "merchant", "application", "session"
	ID   string
}

// NoTarget is used for actions that affect nothing in particular, such as a
// failed sign-in for an address that has no account.
var NoTarget = Target{Kind: "none", ID: "-"}

// Source describes where a request came from.
type Source struct {
	IP        string
	UserAgent string
	RequestID string
}

// Entry is something that happened. It refers to people and records by ID and
// never contains personal data: no names, email addresses or phone numbers.
type Entry struct {
	Actor  Actor
	Action string // a stable name such as "application.approved"
	Target Target
	Reason string            // required when staff act
	Change map[string]string // identifiers and non-personal values
	Source Source
}

// Pending is a validated entry, ready to be chained.
type Pending struct {
	Entry
	At         time.Time
	ChangeJSON string // canonical JSON of Change, as it is hashed and stored
}

// Hash is a SHA-256 chain link.
type Hash [32]byte

// Record is a stored entry.
type Record struct {
	Entry
	Seq        int64
	At         time.Time
	ChangeJSON string
	PrevHash   Hash
	Hash       Hash
}

// Head is the newest entry's position and hash. Copy it to storage that
// cannot be rewritten: it is the checkpoint that proves nothing was trimmed.
type Head struct {
	Seq  int64
	Hash Hash
}

// Break describes where the chain stopped being valid.
type Break struct {
	Seq    int64
	Reason string
}

// VerifyResult reports one verification pass.
type VerifyResult struct {
	Checked         int   // entries examined in this pass
	VerifiedThrough int64 // every entry up to this sequence number is intact
	Broken          *Break
}
