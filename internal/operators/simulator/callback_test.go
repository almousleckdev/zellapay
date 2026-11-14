package simulator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/operators"
)

func dueCallback(t *testing.T) (*Simulator, Callback) {
	t.Helper()
	s, clk := newSim(t)
	_, _ = collect(t, s, "00")
	clk.advance(6 * time.Second)
	due := s.DueCallbacks()
	if len(due) != 1 {
		t.Fatalf("want 1 callback, got %d", len(due))
	}
	return s, due[0]
}

func TestParseCallbackAcceptsAGenuineCallback(t *testing.T) {
	s, cb := dueCallback(t)
	ev, err := s.ParseCallback(context.Background(), callbackRequest(cb))
	if err != nil {
		t.Fatal(err)
	}
	if ev.Ref.OurRef != "ref-00" || ev.Status.State != operators.StateSucceeded ||
		ev.Amount != 10000 || ev.Currency != "XAF" {
		t.Fatalf("unexpected event %+v", ev)
	}
}

func TestParseCallbackRejectsForgeries(t *testing.T) {
	s, cb := dueCallback(t)
	tampered := Callback{Body: append([]byte(nil), cb.Body...), Signature: cb.Signature}
	tampered.Body[len(tampered.Body)-3] ^= 1
	cases := map[string]Callback{
		"tampered body":     tampered,
		"missing signature": {Body: cb.Body},
		"garbage signature": {Body: cb.Body, Signature: "zz-not-hex"},
		"wrong signature":   {Body: cb.Body, Signature: "00" + cb.Signature[2:]},
	}
	for name, c := range cases {
		_, err := s.ParseCallback(context.Background(), callbackRequest(c))
		if !errors.Is(err, operators.ErrInvalidCallback) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestParseCallbackRejectsAnotherOperatorsSecret(t *testing.T) {
	_, cb := dueCallback(t)
	other := New(Config{Secret: []byte("different")}, &testClock{now: epoch})
	_, err := other.ParseCallback(context.Background(), callbackRequest(cb))
	if !errors.Is(err, operators.ErrInvalidCallback) {
		t.Fatalf("got %v", err)
	}
}

func TestParseCallbackRejectsSignedButMalformedBodies(t *testing.T) {
	s, _ := newSim(t)
	for name, body := range map[string]string{
		"not json":      `nope`,
		"unknown field": `{"reference":"r","status":"SUCCESS","extra":1}`,
		"no reference":  `{"status":"SUCCESS"}`,
	} {
		cb := Callback{Body: []byte(body), Signature: s.mac([]byte(body))}
		_, err := s.ParseCallback(context.Background(), callbackRequest(cb))
		if !errors.Is(err, operators.ErrInvalidCallback) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}
