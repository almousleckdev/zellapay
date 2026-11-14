package collections

import "testing"

var allStatuses = []Status{
	StatusCreated, StatusPending, StatusAmbiguous,
	StatusSucceeded, StatusFailed, StatusExpired,
}

func TestTransitionTable(t *testing.T) {
	allowed := map[Status]map[Status]bool{
		StatusCreated: {StatusPending: true, StatusFailed: true, StatusAmbiguous: true},
		StatusPending: {
			StatusSucceeded: true, StatusFailed: true,
			StatusExpired: true, StatusAmbiguous: true,
		},
		StatusAmbiguous: {
			StatusPending: true, StatusSucceeded: true,
			StatusFailed: true, StatusExpired: true,
		},
	}
	for _, from := range allStatuses {
		for _, to := range allStatuses {
			if got, want := from.CanTransitionTo(to), allowed[from][to]; got != want {
				t.Errorf("%s -> %s: got %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestFinalStatesNeverChange(t *testing.T) {
	for _, s := range []Status{StatusSucceeded, StatusFailed, StatusExpired} {
		if !s.Terminal() || s.Open() {
			t.Errorf("%s must be terminal and not open", s)
		}
		for _, to := range allStatuses {
			if s.CanTransitionTo(to) {
				t.Errorf("%s must not move to %s", s, to)
			}
		}
	}
}

func TestOpenStates(t *testing.T) {
	if !StatusPending.Open() || !StatusAmbiguous.Open() || StatusCreated.Open() {
		t.Fatal("pending and ambiguous are open; created is not")
	}
}
