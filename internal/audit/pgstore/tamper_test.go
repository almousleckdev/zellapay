package pgstore_test

import (
	"context"
	"fmt"
	"testing"
)

// These tests attack the log the way someone with full database access
// would: they switch the protective triggers off first. The chain must still
// expose every change, which is the whole point of hashing it.
//
// Two checks cover different attacks. The routine pass is cheap: it checks
// what is new plus the last entry it verified before. The full pass walks
// everything and is the only one that finds an edit deep in old history.

func (w *world) withoutTriggers(t *testing.T, tamper func()) {
	t.Helper()
	for _, table := range []string{"audit_log", "audit_head"} {
		if err := w.exec(t, "ALTER TABLE "+table+" DISABLE TRIGGER USER"); err != nil {
			t.Fatal(err)
		}
	}
	tamper()
}

// seeded returns a log of n entries that have all been verified.
func seeded(t *testing.T, n int) *world {
	t.Helper()
	w := isolated(t)
	for i := 0; i < n; i++ {
		w.record(t, "a.step")
	}
	if res := w.verify(t); res.Broken != nil || res.VerifiedThrough != int64(n) {
		t.Fatalf("setup: %+v", res)
	}
	return w
}

func (w *world) verifyFull(t *testing.T) (int64, int64) {
	t.Helper()
	res, err := w.svc.VerifyFull(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.Broken == nil {
		return 0, res.VerifiedThrough
	}
	return res.Broken.Seq, res.VerifiedThrough
}

func TestAnEditDeepInOldHistoryIsFoundByTheFullPass(t *testing.T) {
	w := seeded(t, 5)
	w.withoutTriggers(t, func() {
		if err := w.exec(t, rewrite(2)); err != nil {
			t.Fatal(err)
		}
	})
	if res := w.verify(t); res.Broken != nil {
		t.Fatalf("the routine pass trusts what it already verified, got %+v", res)
	}
	if broken, through := w.verifyFull(t); broken != 2 || through != 1 {
		t.Fatalf("the full pass must find entry 2: broken=%d through=%d", broken, through)
	}
}

func TestAnEditToTheNewestVerifiedEntryIsFoundByTheRoutinePass(t *testing.T) {
	w := seeded(t, 5)
	w.withoutTriggers(t, func() {
		if err := w.exec(t, rewrite(5)); err != nil {
			t.Fatal(err)
		}
	})
	if res := w.verify(t); res.Broken == nil || res.Broken.Seq != 5 {
		t.Fatalf("got %+v", res)
	}
}

func TestADeletedEntryIsFoundByTheFullPass(t *testing.T) {
	w := seeded(t, 5)
	w.withoutTriggers(t, func() {
		if err := w.exec(t, "DELETE FROM audit_log WHERE seq = 3"); err != nil {
			t.Fatal(err)
		}
	})
	if broken, through := w.verifyFull(t); broken != 3 || through != 2 {
		t.Fatalf("broken=%d through=%d", broken, through)
	}
}

func TestATrimmedTailIsFoundByTheRoutinePass(t *testing.T) {
	w := seeded(t, 5)
	w.withoutTriggers(t, func() {
		if err := w.exec(t, "DELETE FROM audit_log WHERE seq > 3"); err != nil {
			t.Fatal(err)
		}
	})
	if res := w.verify(t); res.Broken == nil {
		t.Fatalf("the head still says 5 entries exist: %+v", res)
	}
}

func TestAForgedHeadIsFound(t *testing.T) {
	w := seeded(t, 3)
	w.withoutTriggers(t, func() {
		if err := w.exec(t, "UPDATE audit_head SET hash = '\\x"+zeros()+"'"); err != nil {
			t.Fatal(err)
		}
	})
	if res := w.verify(t); res.Broken == nil {
		t.Fatalf("got %+v", res)
	}
}

func TestABreakStaysReportedAcrossRestarts(t *testing.T) {
	w := seeded(t, 4)
	w.withoutTriggers(t, func() {
		if err := w.exec(t, rewrite(4)); err != nil {
			t.Fatal(err)
		}
	})
	if res := w.verify(t); res.Broken == nil || res.Broken.Seq != 4 {
		t.Fatalf("got %+v", res)
	}
	restarted := newWorldOn(w.pool) // a fresh service, as after a restart
	if res := restarted.verify(t); res.Broken == nil || res.Broken.Seq != 4 {
		t.Fatalf("the alarm must survive a restart: %+v", res)
	}
}

func TestVerificationOnlyChecksWhatIsNew(t *testing.T) {
	w := seeded(t, 3)
	w.record(t, "a.more")
	w.record(t, "a.more")
	res := w.verify(t)
	if res.Broken != nil || res.Checked != 2 || res.VerifiedThrough != 5 {
		t.Fatalf("got %+v", res)
	}
}

func TestAnIntactLogPassesTheFullPass(t *testing.T) {
	w := seeded(t, 7)
	if broken, through := w.verifyFull(t); broken != 0 || through != 7 {
		t.Fatalf("broken=%d through=%d", broken, through)
	}
}

// rewrite is SQL that silently changes the content of one entry.
func rewrite(seq int) string {
	return fmt.Sprintf("UPDATE audit_log SET reason = 'rewritten later' WHERE seq = %d", seq)
}

func zeros() string {
	b := make([]byte, 64)
	for i := range b {
		b[i] = '0'
	}
	return string(b)
}
