package merchants

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func seedMerchants(t *testing.T, e env, n int) {
	t.Helper()
	for i := range n {
		e.clock.now = e.clock.now.Add(time.Minute)
		if _, err := e.svc.CreateMerchant(
			context.Background(), fmt.Sprintf("Shop %02d", i), "XAF",
		); err != nil {
			t.Fatal(err)
		}
	}
}

func TestListPagesNewestFirstWithoutGapsOrRepeats(t *testing.T) {
	e := newEnv(t, ModeTest)
	seedMerchants(t, e, 6) // plus the "Acme" merchant: 7 in all
	seen := map[string]bool{}
	q := ListQuery{Limit: 3}
	for pages := 0; ; pages++ {
		p, err := e.svc.List(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range p.Merchants {
			if seen[m.ID] {
				t.Fatalf("%s listed twice", m.ID)
			}
			seen[m.ID] = true
		}
		if p.Next == nil {
			break
		}
		if pages > 5 {
			t.Fatal("paging does not end")
		}
		q.After = p.Next
	}
	if len(seen) != 7 {
		t.Fatalf("listed %d merchants, want 7", len(seen))
	}
}

func TestListFiltersAndRefusesBadQueries(t *testing.T) {
	e := newEnv(t, ModeTest)
	seedMerchants(t, e, 2)
	p, err := e.svc.List(context.Background(), ListQuery{Status: StatusActive})
	if err != nil || len(p.Merchants) != 1 {
		t.Fatalf("active = %d, %v", len(p.Merchants), err)
	}
	p, err = e.svc.List(context.Background(), ListQuery{Search: "shop 01"})
	if err != nil || len(p.Merchants) != 1 {
		t.Fatalf("search = %d, %v", len(p.Merchants), err)
	}
	for _, bad := range []ListQuery{{Status: "gone"}, {Limit: MaxPageSize + 1}, {Limit: -1}} {
		if _, err := e.svc.List(context.Background(), bad); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%+v = %v", bad, err)
		}
	}
}

func TestDetailIncludesFees(t *testing.T) {
	e := newEnv(t, ModeTest)
	if err := e.svc.SetFeeBPS(context.Background(), e.mer.ID, 150); err != nil {
		t.Fatal(err)
	}
	d, err := e.svc.Detail(context.Background(), e.mer.ID)
	if err != nil || d.FeeBPS != 150 || d.Name != "Acme" {
		t.Fatalf("detail = %+v, %v", d, err)
	}
	if _, err := e.svc.Detail(context.Background(), "mer_x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing = %v", err)
	}
}
