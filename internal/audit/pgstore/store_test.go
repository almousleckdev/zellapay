package pgstore_test

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/zellapay/zellapay/internal/audit"
)

func TestEntriesFormAChainInTheDatabase(t *testing.T) {
	w := isolated(t)
	first, second := w.record(t, "a.one"), w.record(t, "a.two")
	chained := first.Seq == 1 && first.PrevHash == audit.Genesis &&
		second.Seq == 2 && second.PrevHash == first.Hash
	if !chained {
		t.Fatalf("not chained: %+v %+v", first, second)
	}
	head, err := w.svc.Head(context.Background())
	if err != nil || head.Seq != 2 || head.Hash != second.Hash {
		t.Fatalf("head %+v, %v", head, err)
	}
}

// What is read back must hash to what was stored, or verification would fail
// on honest data: the time and the change survive the round trip exactly.
func TestAStoredEntryStillMatchesItsHashAfterARoundTrip(t *testing.T) {
	w := isolated(t)
	written := w.record(t, "a.roundtrip")
	rows, err := w.store.After(context.Background(), 0, 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("%d rows, %v", len(rows), err)
	}
	got := rows[0]
	if audit.Recompute(got) != got.Hash || got.Hash != written.Hash {
		t.Fatal("a stored entry must recompute to the hash it was written with")
	}
	if !reflect.DeepEqual(got.Change, written.Change) || got.Source.UserAgent != "Mozilla/5.0 é" ||
		!got.At.Equal(written.At) {
		t.Fatalf("the entry changed in storage: %+v vs %+v", got, written)
	}
}

func TestConcurrentWritersProduceOneUnbrokenChain(t *testing.T) {
	w := isolated(t)
	const n = 40
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := w.svc.Record(context.Background(), entry("a.concurrent")); err != nil {
				t.Errorf("record: %v", err)
			}
		}()
	}
	wg.Wait()
	rows, _ := w.store.After(context.Background(), 0, 1000)
	if len(rows) != n {
		t.Fatalf("%d entries, want %d", len(rows), n)
	}
	for i, r := range rows {
		if r.Seq != int64(i+1) {
			t.Fatalf("a gap or duplicate at position %d: seq %d", i, r.Seq)
		}
	}
	if res := w.verify(t); res.Broken != nil || res.VerifiedThrough != n {
		t.Fatalf("got %+v", res)
	}
}

// An entry written inside a transaction exists only if the transaction
// commits, and a rollback must not leave a gap in the numbering.
func TestAnEntryInATransactionFollowsItsFate(t *testing.T) {
	w := isolated(t)
	ctx := context.Background()
	w.record(t, "a.before")

	pending, err := w.svc.Prepare(entry("a.rolled_back"))
	if err != nil {
		t.Fatal(err)
	}
	tx, _ := w.pool.Begin(ctx)
	if _, err := w.store.AppendIn(ctx, tx, pending); err != nil {
		t.Fatal(err)
	}
	_ = tx.Rollback(ctx)

	committed, _ := w.svc.Prepare(entry("a.committed"))
	var rec audit.Record
	if err := pgx.BeginFunc(ctx, w.pool, func(tx pgx.Tx) error {
		var err error
		rec, err = w.store.AppendIn(ctx, tx, committed)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if rec.Seq != 2 {
		t.Fatalf("a rolled-back entry must not use a number: got seq %d", rec.Seq)
	}
	if res := w.verify(t); res.Broken != nil || res.VerifiedThrough != 2 {
		t.Fatalf("got %+v", res)
	}
}

func TestListFiltersAndPaginatesNewestFirst(t *testing.T) {
	w := isolated(t)
	for i := 0; i < 5; i++ {
		w.record(t, "a.listed")
	}
	other := entry("b.other")
	other.Target = audit.Target{Kind: "merchant", ID: "mer_2"}
	other.Actor = audit.Actor{Kind: audit.ActorSystem, ID: "sched"}
	if _, err := w.svc.Record(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	byTarget, _ := w.svc.List(ctx, audit.Filter{TargetKind: "merchant", TargetID: "mer_1"})
	byActor, _ := w.svc.List(ctx, audit.Filter{ActorKind: audit.ActorSystem, ActorID: "sched"})
	byAction, _ := w.svc.List(ctx, audit.Filter{Action: "b.other"})
	if len(byTarget) != 5 || len(byActor) != 1 || len(byAction) != 1 {
		t.Fatalf("target %d, actor %d, action %d", len(byTarget), len(byActor), len(byAction))
	}
	page1, _ := w.svc.List(ctx, audit.Filter{Limit: 4})
	page2, _ := w.svc.List(ctx, audit.Filter{Limit: 4, BeforeSeq: page1[len(page1)-1].Seq})
	if page1[0].Seq != 6 || len(page1) != 4 || page2[0].Seq != 2 || len(page2) != 2 {
		t.Fatalf("pages: %d..%d then %d..%d",
			page1[0].Seq, page1[3].Seq, page2[0].Seq, page2[len(page2)-1].Seq)
	}
}
