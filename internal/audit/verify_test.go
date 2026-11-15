package audit

import (
	"context"
	"testing"
)

func seeded(t *testing.T, n int) (*Service, *memRepo) {
	t.Helper()
	s, repo := newService()
	for i := 0; i < n; i++ {
		s.must(t, "a.step")
	}
	return s, repo
}

func verify(t *testing.T, s *Service, limit int) VerifyResult {
	t.Helper()
	res, err := s.Verify(context.Background(), limit)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestAnIntactChainVerifiesAndVerificationIsIncremental(t *testing.T) {
	s, _ := seeded(t, 5)
	first := verify(t, s, 100)
	if first.Broken != nil || first.Checked != 5 || first.VerifiedThrough != 5 {
		t.Fatalf("got %+v", first)
	}
	s.must(t, "a.more")
	second := verify(t, s, 100)
	if second.Checked != 1 || second.VerifiedThrough != 6 {
		t.Fatalf("only the new entry must be examined: %+v", second)
	}
}

func TestVerificationWorksInSmallBatches(t *testing.T) {
	s, _ := seeded(t, 7)
	var through int64
	for i := 0; i < 10 && through < 7; i++ {
		res := verify(t, s, 3)
		if res.Broken != nil {
			t.Fatalf("batch %d: %+v", i, res)
		}
		through = res.VerifiedThrough
	}
	if through != 7 {
		t.Fatalf("verified through %d", through)
	}
}

func TestAnEditedEntryIsFound(t *testing.T) {
	s, repo := seeded(t, 5)
	repo.rows[2].Reason = "rewritten after the fact"
	res := verify(t, s, 100)
	if res.Broken == nil || res.Broken.Seq != 3 || res.VerifiedThrough != 2 {
		t.Fatalf("got %+v", res)
	}
}

func TestADeletedEntryIsFound(t *testing.T) {
	s, repo := seeded(t, 5)
	repo.rows = append(repo.rows[:1], repo.rows[2:]...)
	res := verify(t, s, 100)
	if res.Broken == nil || res.Broken.Seq != 2 {
		t.Fatalf("got %+v", res)
	}
}

func TestATrimmedTailIsFound(t *testing.T) {
	s, repo := seeded(t, 5)
	repo.rows = repo.rows[:3]
	res := verify(t, s, 100)
	if res.Broken == nil || res.Broken.Seq != 4 {
		t.Fatalf("got %+v", res)
	}
}

func TestAForgedHeadIsFound(t *testing.T) {
	s, repo := seeded(t, 3)
	repo.head.Hash[0] ^= 0xff
	if res := verify(t, s, 100); res.Broken == nil {
		t.Fatalf("got %+v", res)
	}
}

// staleHead answers Head from before the newest append, as happens when an
// entry is added between reading the head and reading the entries.
type staleHead struct {
	*memRepo
	head Head
}

func (s staleHead) Head(context.Context) (Head, error) { return s.head, nil }

func TestEntriesAppendedWhileVerifyingAreNotMistakenForTampering(t *testing.T) {
	_, repo := seeded(t, 5)
	snapshot := repo.head
	newer := NewService(repo, fixedClock{})
	newer.must(t, "a.late") // appended after the head was read
	racing := NewService(staleHead{memRepo: repo, head: snapshot}, fixedClock{})
	res := verify(t, racing, 100)
	if res.Broken != nil || res.VerifiedThrough != 6 {
		t.Fatalf("an entry newer than the head snapshot is not tampering: %+v", res)
	}
}

func TestABreakIsRememberedUntilAPersonResolvesIt(t *testing.T) {
	s, repo := seeded(t, 4)
	repo.rows[1].Action = "a.tampered"
	first := verify(t, s, 100)
	if first.Broken == nil {
		t.Fatal("expected a break")
	}
	repo.rows[1].Action = "a.step" // quietly repairing it must not clear the alarm
	again := verify(t, s, 100)
	if again.Broken == nil || again.Broken.Seq != first.Broken.Seq {
		t.Fatalf("the break must persist: %+v", again)
	}
}

func TestTheRoutinePassTrustsHistoryButChecksTheNewestVerifiedEntry(t *testing.T) {
	s, repo := seeded(t, 5)
	if res := verify(t, s, 100); res.Broken != nil {
		t.Fatalf("setup: %+v", res)
	}
	repo.rows[1].Reason = "an old entry rewritten" // deep in verified history
	if res := verify(t, s, 100); res.Broken != nil {
		t.Fatalf("the routine pass does not re-read old history: %+v", res)
	}
	repo.rows[4].Reason = "the newest verified entry rewritten"
	if res := verify(t, s, 100); res.Broken == nil || res.Broken.Seq != 5 {
		t.Fatalf("the routine pass must re-check the newest verified entry: %+v", res)
	}
}

func TestTheFullPassFindsAnEditAnywhere(t *testing.T) {
	s, repo := seeded(t, 6)
	verify(t, s, 100)
	repo.rows[1].Action = "a.tampered"
	res, err := s.VerifyFull(context.Background(), 2)
	if err != nil || res.Broken == nil || res.Broken.Seq != 2 || res.VerifiedThrough != 1 {
		t.Fatalf("got %+v, %v", res, err)
	}
}

func TestAnIntactLogPassesTheFullPassInBatches(t *testing.T) {
	s, _ := seeded(t, 7)
	res, err := s.VerifyFull(context.Background(), 3)
	if err != nil || res.Broken != nil || res.VerifiedThrough != 7 || res.Checked != 7 {
		t.Fatalf("got %+v, %v", res, err)
	}
}
