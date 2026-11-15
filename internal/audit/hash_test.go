package audit

import (
	"testing"
	"time"
)

var at = time.Date(2026, 11, 14, 12, 0, 0, 123456000, time.UTC)

func pending() Pending {
	return Pending{
		Entry: Entry{
			Actor: Actor{ActorStaff, "stf_1"}, Action: "application.approved",
			Target: Target{"application", "app_1"}, Reason: "documents verified",
			Source: Source{IP: "203.0.113.7", UserAgent: "Mozilla", RequestID: "req_1"},
		},
		At: at, ChangeJSON: `{"fee_bps":"150"}`,
	}
}

func TestLinkIsDeterministic(t *testing.T) {
	first, second := Link(Genesis, 1, pending()), Link(Genesis, 1, pending())
	if first != second {
		t.Fatal("the same input must give the same hash")
	}
}

func TestChangingAnyFieldChangesTheHash(t *testing.T) {
	base := Link(Genesis, 1, pending())
	mutations := map[string]func(*Pending){
		"actor kind":  func(p *Pending) { p.Actor.Kind = ActorSystem },
		"actor id":    func(p *Pending) { p.Actor.ID = "stf_2" },
		"action":      func(p *Pending) { p.Action = "application.rejected" },
		"target kind": func(p *Pending) { p.Target.Kind = "merchant" },
		"target id":   func(p *Pending) { p.Target.ID = "app_2" },
		"reason":      func(p *Pending) { p.Reason = "different" },
		"change":      func(p *Pending) { p.ChangeJSON = `{"fee_bps":"151"}` },
		"ip":          func(p *Pending) { p.Source.IP = "203.0.113.8" },
		"user agent":  func(p *Pending) { p.Source.UserAgent = "Other" },
		"request id":  func(p *Pending) { p.Source.RequestID = "req_2" },
		"time":        func(p *Pending) { p.At = p.At.Add(time.Microsecond) },
	}
	for name, mutate := range mutations {
		p := pending()
		mutate(&p)
		if Link(Genesis, 1, p) == base {
			t.Errorf("changing the %s must change the hash", name)
		}
	}
	if Link(Genesis, 2, pending()) == base {
		t.Error("the position must be part of the hash")
	}
	var other Hash
	other[0] = 1
	if Link(other, 1, pending()) == base {
		t.Error("the previous hash must be part of the hash")
	}
}

func TestValuesCannotBeShiftedBetweenFields(t *testing.T) {
	a, b := pending(), pending()
	a.Action, a.Reason = "x.ab", "c"
	b.Action, b.Reason = "x.a", "bc"
	if Link(Genesis, 1, a) == Link(Genesis, 1, b) {
		t.Fatal("moving text from one field to the next must change the hash")
	}
}

func TestCanonicalChangeIgnoresMapOrder(t *testing.T) {
	first, _ := canonicalChange(map[string]string{"b": "2", "a": "1", "c": "3"})
	for i := 0; i < 20; i++ {
		again, _ := canonicalChange(map[string]string{"c": "3", "a": "1", "b": "2"})
		if again != first {
			t.Fatalf("%s != %s", again, first)
		}
	}
	if first != `{"a":"1","b":"2","c":"3"}` {
		t.Fatalf("got %s", first)
	}
	if empty, _ := canonicalChange(nil); empty != "{}" {
		t.Fatalf("an empty change is %q", empty)
	}
}

func TestStampTimeMatchesWhatTheDatabaseKeeps(t *testing.T) {
	t0 := time.Date(2026, 11, 14, 12, 0, 0, 123456789, time.FixedZone("x", 3600))
	got := StampTime(t0)
	if got.Nanosecond() != 123456000 || got.Location() != time.UTC {
		t.Fatalf("got %v", got)
	}
}
