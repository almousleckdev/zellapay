// Package notify tells people things by email or SMS: invitations,
// verification links, password resets and security alerts. Every message goes
// through a transactional outbox, so the event that caused it and the message
// are stored together and a mail server outage never fails a sign-up.
package notify

import "time"

// Channel is how a message reaches a person.
type Channel string

// Channels.
const (
	Email Channel = "email"
	SMS   Channel = "sms"
)

// Locales messages are written in. French is the primary language.
const (
	LocaleFrench  = "fr"
	LocaleEnglish = "en"
)

// Message is something to tell one person.
type Message struct {
	To        string            // an email address or an E.164 phone number
	Template  string            // a name from the catalogue
	Locale    string            // "fr" or "en"
	Data      map[string]string // exactly the keys the template declares
	DedupeKey string            // optional: a repeat with the same key is ignored
}

// Delivery is a rendered message handed to a Sender.
type Delivery struct {
	ID      string
	To      string
	Subject string // email only
	Body    string
}

// Prepared is a validated message ready to store: the recipient and data are
// sealed, so the database never holds an address, a link or a code in clear.
type Prepared struct {
	ID              string
	Channel         Channel
	Template        string
	Locale          string
	RecipientMasked string
	PayloadSealed   string
	DedupeKey       string
	CreatedAt       time.Time
	RetryUntil      time.Time
}

// Status of a stored message.
type Status string

// Message states.
const (
	StatusPending Status = "pending"
	StatusSent    Status = "sent"
	StatusDead    Status = "dead" // gave up; the payload has been erased
)

// Queued is a stored message claimed for sending.
type Queued struct {
	ID            string
	Channel       Channel
	Template      string
	Locale        string
	PayloadSealed string
	Attempts      int
	RetryUntil    time.Time
}

// Result is the new state of a message after an attempt.
type Result struct {
	Status        Status
	Attempts      int
	NextAttemptAt *time.Time
	LastError     string
	SentAt        *time.Time
}

// Record is a stored message's public state, without its payload.
type Record struct {
	ID              string
	Channel         Channel
	Template        string
	RecipientMasked string
	Status          Status
	Attempts        int
	LastError       string
	CreatedAt       time.Time
	SentAt          *time.Time
	PayloadErased   bool
}
