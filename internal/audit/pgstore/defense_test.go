package pgstore_test

import (
	"testing"
)

// The database itself refuses to alter history, whatever the application does.
func TestTheDatabaseRefusesToAlterTheLog(t *testing.T) {
	w := isolated(t)
	w.record(t, "a.one")
	w.record(t, "a.two")
	for name, sql := range map[string]string{
		"update":   "UPDATE audit_log SET reason = 'edited' WHERE seq = 1",
		"delete":   "DELETE FROM audit_log WHERE seq = 1",
		"truncate": "TRUNCATE audit_log",
	} {
		if err := w.exec(t, sql); err == nil {
			t.Errorf("%s must be rejected", name)
		}
	}
	if res := w.verify(t); res.Broken != nil || res.VerifiedThrough != 2 {
		t.Fatalf("the log must be untouched: %+v", res)
	}
}

// An entry that does not extend the head is refused, so the chain cannot fork.
func TestTheDatabaseRefusesAnEntryThatDoesNotExtendTheChain(t *testing.T) {
	w := isolated(t)
	first := w.record(t, "a.one")
	const insert = `INSERT INTO audit_log
		(seq, at, actor_kind, actor_id, action, target_kind, target_id, prev_hash, hash)
		VALUES ($1, now(), 'system', 's', 'a.x', 'k', 'i', $2, $3)`
	zero := make([]byte, 32)
	forged := make([]byte, 32)
	forged[0] = 9
	for name, args := range map[string][]any{
		"wrong sequence": {int64(5), first.Hash[:], forged},
		"duplicate seq":  {int64(1), first.Hash[:], forged},
		"wrong previous": {int64(2), zero, forged},
	} {
		if err := w.exec(t, insert, args...); err == nil {
			t.Errorf("%s must be rejected", name)
		}
	}
}

func TestTheHeadCannotBeMovedToAnEntryThatDoesNotExist(t *testing.T) {
	w := isolated(t)
	w.record(t, "a.one")
	forged := make([]byte, 32)
	forged[0] = 7
	for name, sql := range map[string]string{
		"skip ahead": "UPDATE audit_head SET seq = 5, hash = $1",
		"fake hash":  "UPDATE audit_head SET seq = 2, hash = $1",
		"rewind":     "UPDATE audit_head SET seq = 0, hash = $1",
	} {
		if err := w.exec(t, sql, forged); err == nil {
			t.Errorf("%s must be rejected", name)
		}
	}
}
