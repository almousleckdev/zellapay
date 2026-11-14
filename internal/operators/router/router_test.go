package router

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/zellapay/zellapay/internal/operators"
)

type stub struct{ code string }

func (s stub) Code() string { return s.code }
func (stub) InitiateCollection(
	context.Context, operators.CollectionRequest,
) (operators.Ref, error) {
	return operators.Ref{}, nil
}
func (stub) InitiatePayout(context.Context, operators.PayoutRequest) (operators.Ref, error) {
	return operators.Ref{}, nil
}
func (stub) Status(context.Context, operators.Ref) (operators.Status, error) {
	return operators.Status{}, nil
}
func (stub) ParseCallback(context.Context, *http.Request) (operators.CallbackEvent, error) {
	return operators.CallbackEvent{}, nil
}
func (stub) Health(context.Context) operators.Health { return operators.Health{OK: true} }

func newRouter(t *testing.T, rules ...Rule) *Router {
	t.Helper()
	r, err := New(rules, stub{"a"}, stub{"b"})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRouteUsesTheLongestPrefix(t *testing.T) {
	r := newRouter(t, Rule{"+2410", "a"}, Rule{"+24107", "b"})
	cases := map[string]string{"+24107123456": "b", "+24104123456": "a"}
	for phone, want := range cases {
		got, err := r.Route(phone)
		if err != nil || got.Code() != want {
			t.Errorf("%s: got %v, %v; want %s", phone, got, err, want)
		}
	}
}

func TestRouteReportsNoMatch(t *testing.T) {
	r := newRouter(t, Rule{"+24107", "a"})
	if _, err := r.Route("+33612345678"); !errors.Is(err, ErrNoRoute) {
		t.Fatalf("got %v", err)
	}
}

func TestNewValidatesRules(t *testing.T) {
	if _, err := New([]Rule{{"24107", "a"}}, stub{"a"}); err == nil {
		t.Error("prefix without + must be rejected")
	}
	if _, err := New([]Rule{{"+24107", "missing"}}, stub{"a"}); err == nil {
		t.Error("unknown operator must be rejected")
	}
}

func TestAdapterLookupByCode(t *testing.T) {
	r := newRouter(t)
	if a, ok := r.Adapter("a"); !ok || a.Code() != "a" {
		t.Fatal("registered adapter must be found")
	}
	if _, ok := r.Adapter("zzz"); ok {
		t.Fatal("unknown adapter must not be found")
	}
}
