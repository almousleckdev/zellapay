// Package collections requests money from a customer's phone through a
// mobile money operator and records the result in the ledger.
package collections

// Status is the lifecycle state of a collection.
type Status string

// Collection states. Succeeded, failed and expired are final.
const (
	StatusCreated   Status = "created"   // stored, operator not yet asked
	StatusPending   Status = "pending"   // waiting for the customer
	StatusAmbiguous Status = "ambiguous" // outcome unknown, being resolved
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusExpired   Status = "expired" // the operator gave up waiting
)

var transitions = map[Status][]Status{
	StatusCreated:   {StatusPending, StatusFailed, StatusAmbiguous},
	StatusPending:   {StatusSucceeded, StatusFailed, StatusExpired, StatusAmbiguous},
	StatusAmbiguous: {StatusPending, StatusSucceeded, StatusFailed, StatusExpired},
}

// Terminal reports whether s can never change again.
func (s Status) Terminal() bool {
	return s == StatusSucceeded || s == StatusFailed || s == StatusExpired
}

// Open reports whether the operator may still change the outcome.
func (s Status) Open() bool { return s == StatusPending || s == StatusAmbiguous }

// CanTransitionTo reports whether the state machine allows from to.
func (s Status) CanTransitionTo(to Status) bool {
	for _, allowed := range transitions[s] {
		if allowed == to {
			return true
		}
	}
	return false
}
