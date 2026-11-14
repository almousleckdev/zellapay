package payouts

import "time"

// transition returns p moved to the given state, with the audit event.
func (p Payout) transition(
	to Status, evt, detail string, now time.Time,
) (Payout, Event, error) {
	if !p.Status.CanTransitionTo(to) {
		return p, Event{}, ErrInvalidTransition
	}
	ev := Event{Type: evt, From: p.Status, To: to, Detail: detail, At: now}
	p.Status = to
	p.UpdatedAt = now
	if to.Terminal() {
		p.CompletedAt = &now
		p.NextCheckAt = nil
	}
	return p, ev, nil
}

// withCheck returns p scheduled to be checked again at when.
func (p Payout) withCheck(when, now time.Time) Payout {
	p.UpdatedAt = now
	p.NextCheckAt = &when
	return p
}
