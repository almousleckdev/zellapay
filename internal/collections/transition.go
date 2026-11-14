package collections

import "time"

// transition returns c moved to the given state, with the audit event.
func (c Collection) transition(
	to Status, evt, detail string, now time.Time,
) (Collection, Event, error) {
	if !c.Status.CanTransitionTo(to) {
		return c, Event{}, ErrInvalidTransition
	}
	ev := Event{Type: evt, From: c.Status, To: to, Detail: detail, At: now}
	c.Status = to
	c.UpdatedAt = now
	if to.Terminal() {
		c.CompletedAt = &now
		c.NextCheckAt = nil
	}
	return c, ev, nil
}

// withCheck returns c scheduled to be checked again at when.
func (c Collection) withCheck(when time.Time, now time.Time) Collection {
	c.UpdatedAt = now
	c.NextCheckAt = &when
	return c
}
