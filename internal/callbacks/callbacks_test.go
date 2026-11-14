package callbacks

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zellapay/zellapay/internal/operators"
)

// stubAdapter returns a canned callback event.
type stubAdapter struct {
	operators.Adapter
	event operators.CallbackEvent
	err   error
}

func (s stubAdapter) ParseCallback(
	context.Context, *http.Request,
) (operators.CallbackEvent, error) {
	return s.event, s.err
}

type stubAdapters map[string]operators.Adapter

func (s stubAdapters) Adapter(code string) (operators.Adapter, bool) {
	a, ok := s[code]
	return a, ok
}

type stubSink struct {
	prefix string
	err    error
	got    []string
}

func (s *stubSink) Owns(ref string) bool { return strings.HasPrefix(ref, s.prefix) }

func (s *stubSink) HandleEvent(_ context.Context, code string, ev operators.CallbackEvent) error {
	s.got = append(s.got, code+":"+ev.Ref.OurRef)
	return s.err
}

func event(ref string) operators.CallbackEvent {
	return operators.CallbackEvent{Ref: operators.Ref{OurRef: ref}}
}

func newDispatcher(a stubAdapter, sinks ...Sink) *Dispatcher {
	return New(stubAdapters{"sim": a}, sinks...)
}

func call(d *Dispatcher, operator string) int {
	mux := http.NewServeMux()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux.Handle("POST /callbacks/{operator}", d.Handler(log))
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/callbacks/"+operator, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec.Code
}

func TestACallbackGoesToTheModuleThatOwnsTheReference(t *testing.T) {
	cols, pays := &stubSink{prefix: "col_"}, &stubSink{prefix: "pay_"}
	d := newDispatcher(stubAdapter{event: event("pay_1")}, cols, pays)
	if code := call(d, "sim"); code != http.StatusNoContent {
		t.Fatalf("got %d", code)
	}
	if len(cols.got) != 0 || len(pays.got) != 1 || pays.got[0] != "sim:pay_1" {
		t.Fatalf("collections=%v payouts=%v", cols.got, pays.got)
	}
}

func TestStatusMapping(t *testing.T) {
	boom := errors.New("database down")
	cases := []struct {
		name     string
		adapter  stubAdapter
		sink     error
		operator string
		want     int
	}{
		{"handled", stubAdapter{event: event("col_1")}, nil, "sim", 204},
		{"nobody owns it", stubAdapter{event: event("zzz_1")}, nil, "sim", 204},
		{"sink lacks it", stubAdapter{event: event("col_1")}, ErrUnknownReference, "sim", 204},
		{"unknown operator", stubAdapter{event: event("col_1")}, nil, "nobody", 404},
		{"forged callback", stubAdapter{err: operators.ErrInvalidCallback}, nil, "sim", 400},
		{"sink fails", stubAdapter{event: event("col_1")}, boom, "sim", 500},
	}
	for _, c := range cases {
		d := newDispatcher(c.adapter, &stubSink{prefix: "col_", err: c.sink})
		if got := call(d, c.operator); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}
