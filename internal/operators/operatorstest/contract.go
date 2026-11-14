// Package operatorstest holds the behaviour every operators.Adapter must
// have. Run it from each adapter's tests. It must only be imported by tests.
package operatorstest

import (
	"context"
	"errors"
	"testing"

	"github.com/zellapay/zellapay/internal/operators"
)

// Suite describes the adapter under test.
type Suite struct {
	// New returns a fresh adapter for each subtest.
	New func(t *testing.T) operators.Adapter
	// Phone is a number for which collections and payouts are accepted.
	Phone string
}

func (s Suite) collection(ref string, amount int64) operators.CollectionRequest {
	return operators.CollectionRequest{
		Reference: ref, Phone: s.Phone, Amount: amount, Currency: "XAF",
	}
}

// Run executes the contract.
func Run(t *testing.T, s Suite) {
	t.Run("identifies itself", func(t *testing.T) {
		if s.New(t).Code() == "" {
			t.Fatal("Code must not be empty")
		}
	})
	t.Run("echoes our reference", s.echoesReference)
	t.Run("initiation is idempotent", s.idempotentInitiation)
	t.Run("rejects a reused reference with different data", s.rejectsReusedReference)
	t.Run("unknown reference is not found", s.unknownReference)
	t.Run("payouts follow the same rules", s.payouts)
	t.Run("reports health", func(t *testing.T) {
		if !s.New(t).Health(context.Background()).OK {
			t.Fatal("a fresh adapter must be healthy")
		}
	})
}

func (s Suite) echoesReference(t *testing.T) {
	a := s.New(t)
	ref, err := a.InitiateCollection(context.Background(), s.collection("ref-1", 1000))
	if err != nil || ref.OurRef != "ref-1" {
		t.Fatalf("ref=%+v err=%v", ref, err)
	}
	if _, err := a.Status(context.Background(), ref); err != nil {
		t.Fatalf("status of an initiated reference: %v", err)
	}
}

func (s Suite) idempotentInitiation(t *testing.T) {
	a := s.New(t)
	ctx := context.Background()
	first, err1 := a.InitiateCollection(ctx, s.collection("ref-2", 1000))
	second, err2 := a.InitiateCollection(ctx, s.collection("ref-2", 1000))
	if first != second || !sameOutcome(err1, err2) {
		t.Fatalf("repeat differs: %+v/%v vs %+v/%v", first, err1, second, err2)
	}
}

func (s Suite) rejectsReusedReference(t *testing.T) {
	a := s.New(t)
	ctx := context.Background()
	if _, err := a.InitiateCollection(ctx, s.collection("ref-3", 1000)); err != nil {
		t.Fatal(err)
	}
	_, err := a.InitiateCollection(ctx, s.collection("ref-3", 2000))
	if !errors.Is(err, operators.ErrDuplicateReference) {
		t.Fatalf("got %v", err)
	}
}

func (s Suite) unknownReference(t *testing.T) {
	_, err := s.New(t).Status(context.Background(), operators.Ref{OurRef: "never-sent"})
	if !errors.Is(err, operators.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func (s Suite) payouts(t *testing.T) {
	a := s.New(t)
	ctx := context.Background()
	req := operators.PayoutRequest{Reference: "pay-1", Phone: s.Phone, Amount: 500, Currency: "XAF"}
	first, err1 := a.InitiatePayout(ctx, req)
	second, err2 := a.InitiatePayout(ctx, req)
	if first.OurRef != "pay-1" || first != second || !sameOutcome(err1, err2) {
		t.Fatalf("payout repeat differs: %+v/%v vs %+v/%v", first, err1, second, err2)
	}
}

// sameOutcome compares errors by kind, so two timeouts count as equal.
func sameOutcome(a, b error) bool {
	return errors.Is(a, operators.ErrUnknownOutcome) == errors.Is(b, operators.ErrUnknownOutcome) &&
		(a == nil) == (b == nil)
}
