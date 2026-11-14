package ledger

import (
	"errors"
	"testing"
)

func sample() Journal {
	return Journal{
		ID: "j1", IdempotencyKey: "k1", Kind: "collection", Reference: "r1",
		Entries: []Entry{
			{"a", Debit, 100, "XAF"},
			{"b", Credit, 60, "XAF"},
			{"c", Credit, 40, "XAF"},
		},
	}
}

func TestJournalValidate(t *testing.T) {
	if err := sample().Validate(); err != nil {
		t.Fatal(err)
	}
	j := sample()
	j.Reference = ""
	if err := j.Validate(); !errors.Is(err, ErrInvalidJournal) {
		t.Fatalf("got %v", err)
	}
}

func TestFingerprintIgnoresIDAndOrder(t *testing.T) {
	a, b := sample(), sample()
	b.ID = "other"
	b.Entries[0], b.Entries[2] = b.Entries[2], b.Entries[0]
	if a.Fingerprint() != b.Fingerprint() {
		t.Fatal("equivalent journals must share a fingerprint")
	}
}

func TestFingerprintDetectsChange(t *testing.T) {
	a, b := sample(), sample()
	b.Entries[1].Amount, b.Entries[2].Amount = 61, 39
	if a.Fingerprint() == b.Fingerprint() {
		t.Fatal("different journals must differ")
	}
}

func TestNormalBalance(t *testing.T) {
	if NormalBalance(Asset, 100, 30) != 70 {
		t.Fatal("asset")
	}
	if NormalBalance(Liability, 30, 100) != 70 {
		t.Fatal("liability")
	}
}
