// Package payouts sends money from a merchant's balance to a phone number
// through a mobile money operator. Funds are held before anything is sent,
// captured on success and released on failure.
package payouts

// Status is the lifecycle state of a payout.
type Status string

// Payout states. Succeeded, failed and rejected are final.
const (
	StatusCreated   Status = "created"   // stored; funds not yet held
	StatusHeld      Status = "held"      // funds reserved; operator not yet asked
	StatusPending   Status = "pending"   // sent; waiting for the operator
	StatusAmbiguous Status = "ambiguous" // outcome unknown; funds stay held
	StatusSucceeded Status = "succeeded" // money left; hold captured
	StatusFailed    Status = "failed"    // money did not leave; hold released
	StatusRejected  Status = "rejected"  // refused before holding (no funds)
)

var transitions = map[Status][]Status{
	StatusCreated:   {StatusHeld, StatusRejected, StatusFailed},
	StatusHeld:      {StatusPending, StatusFailed, StatusAmbiguous},
	StatusPending:   {StatusSucceeded, StatusFailed, StatusAmbiguous},
	StatusAmbiguous: {StatusPending, StatusSucceeded, StatusFailed},
}

// Terminal reports whether s can never change again.
func (s Status) Terminal() bool {
	return s == StatusSucceeded || s == StatusFailed || s == StatusRejected
}

// Open reports whether the operator may still change the outcome.
func (s Status) Open() bool {
	return s == StatusHeld || s == StatusPending || s == StatusAmbiguous
}

// CanTransitionTo reports whether the state machine allows from to.
func (s Status) CanTransitionTo(to Status) bool {
	for _, allowed := range transitions[s] {
		if allowed == to {
			return true
		}
	}
	return false
}
