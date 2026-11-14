package collections

import (
	"testing"

	"github.com/zellapay/zellapay/internal/ledger"
)

func succeeded(amount, fee int64) Collection {
	return Collection{
		ID: "col_1", MerchantID: "mer_1", Operator: "airtel_money",
		Currency: "XAF", Amount: amount, Fee: fee,
	}
}

func TestJournalBalancesForEveryFeeShape(t *testing.T) {
	cases := []struct {
		name        string
		amount, fee int64
		wantEntries int
	}{
		{"with a fee", 10000, 150, 3},
		{"without a fee", 10000, 0, 2},
		{"fee equals the amount", 500, 500, 2},
	}
	for _, c := range cases {
		j := succeeded(c.amount, c.fee).journal()
		if err := j.Validate(); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
		if len(j.Entries) != c.wantEntries {
			t.Errorf("%s: %d entries, want %d", c.name, len(j.Entries), c.wantEntries)
		}
	}
}

func TestJournalAccountsAndIdempotencyKey(t *testing.T) {
	j := succeeded(10000, 150).journal()
	want := map[string]struct {
		side   ledger.Side
		amount int64
	}{
		"operator:airtel_money:settlement": {ledger.Debit, 10000},
		"merchant:mer_1:pending":           {ledger.Credit, 9850},
		"zella:fees:XAF":                   {ledger.Credit, 150},
	}
	for _, e := range j.Entries {
		w, ok := want[e.Account]
		if !ok || e.Side != w.side || e.Amount != w.amount {
			t.Errorf("unexpected entry %+v", e)
		}
	}
	if j.IdempotencyKey != "collection:col_1" || j.ID != "jnl_col_1" {
		t.Fatalf("keys must derive from the collection ID: %+v", j)
	}
}

func TestJournalIsTheSameEveryTime(t *testing.T) {
	c := succeeded(10000, 150)
	first, second := c.journal().Fingerprint(), c.journal().Fingerprint()
	if first != second {
		t.Fatal("replaying the posting must present an identical journal")
	}
}
