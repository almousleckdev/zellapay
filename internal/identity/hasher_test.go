package identity

import (
	"strings"
	"testing"
)

var testPepper = []byte(strings.Repeat("p", 32))

func testHasher(t *testing.T, p Params, version int, pepper []byte) *Hasher {
	t.Helper()
	h, err := NewHasher(p, map[int][]byte{version: pepper})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func cheap() Params { return Params{Memory: 8, Passes: 1, Lanes: 1, Version: 1} }

func TestHasher_VerifiesOnlyTheRightPassword(t *testing.T) {
	h := testHasher(t, cheap(), 1, testPepper)
	enc, err := h.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if ok, _, _ := h.Verify("correct horse battery", enc); !ok {
		t.Fatal("right password rejected")
	}
	if ok, _, _ := h.Verify("correct horse batterz", enc); ok {
		t.Fatal("wrong password accepted")
	}
}

func TestHasher_SaltsEveryHash(t *testing.T) {
	h := testHasher(t, cheap(), 1, testPepper)
	a, _ := h.Hash("same password here")
	b, _ := h.Hash("same password here")
	if a == b {
		t.Fatal("two hashes of one password are identical")
	}
}

func TestHasher_TreatsEquivalentUnicodeAsEqual(t *testing.T) {
	h := testHasher(t, cheap(), 1, testPepper)
	enc, _ := h.Hash("café au lait 123")
	if ok, _, _ := h.Verify("café au lait 123", enc); !ok {
		t.Fatal("composed and decomposed forms should match")
	}
}

func TestHasher_FlagsOlderCostForRehash(t *testing.T) {
	old := testHasher(t, cheap(), 1, testPepper)
	enc, _ := old.Hash("a long enough password")
	stronger := Params{Memory: 16, Passes: 2, Lanes: 1, Version: 1}
	h := testHasher(t, stronger, 1, testPepper)
	ok, stale, err := h.Verify("a long enough password", enc)
	if err != nil || !ok || !stale {
		t.Fatalf("ok=%v stale=%v err=%v, want ok and stale", ok, stale, err)
	}
}

func TestHasher_RotatesPepperKeepingOldHashesValid(t *testing.T) {
	old := testHasher(t, cheap(), 1, testPepper)
	enc, _ := old.Hash("a long enough password")
	next := []byte(strings.Repeat("q", 32))
	p := cheap()
	p.Version = 2
	h, err := NewHasher(p, map[int][]byte{1: testPepper, 2: next})
	if err != nil {
		t.Fatal(err)
	}
	ok, stale, _ := h.Verify("a long enough password", enc)
	if !ok || !stale {
		t.Fatalf("ok=%v stale=%v, want old pepper accepted and flagged", ok, stale)
	}
}

func TestHasher_RejectsBadSetupAndMalformedHashes(t *testing.T) {
	if _, err := NewHasher(cheap(), map[int][]byte{1: []byte("short")}); err == nil {
		t.Error("short pepper accepted")
	}
	if _, err := NewHasher(cheap(), map[int][]byte{2: testPepper}); err == nil {
		t.Error("missing current pepper version accepted")
	}
	h := testHasher(t, cheap(), 1, testPepper)
	for _, bad := range []string{"", "plain", "$argon2id$v=19$m=1$x$y", "$bcrypt$a$b$c$d"} {
		if _, _, err := h.Verify("x", bad); err == nil {
			t.Errorf("malformed hash %q accepted", bad)
		}
	}
}
