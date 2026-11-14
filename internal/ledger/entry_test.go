package ledger

import (
	"errors"
	"math"
	"testing"
)

func TestValidate(t *testing.T) {
	d := func(a string, n int64) Entry { return Entry{a, Debit, n, "XAF"} }
	c := func(a string, n int64) Entry { return Entry{a, Credit, n, "XAF"} }

	tests := []struct {
		name    string
		entries []Entry
		want    error
	}{
		{"collection with fee", []Entry{
			d("operator:airtel:settlement", 10000),
			c("merchant:m1:pending", 9850),
			c("zella:fees", 150),
		}, nil},
		{"unbalanced", []Entry{d("a", 100), c("b", 99)}, ErrUnbalanced},
		{"single entry", []Entry{d("a", 100)}, ErrEmptyJournal},
		{"zero amount", []Entry{d("a", 0), c("b", 0)}, ErrInvalidEntry},
		{"negative amount", []Entry{d("a", -5), c("b", -5)}, ErrInvalidEntry},
		{"missing account", []Entry{d("", 5), c("b", 5)}, ErrInvalidEntry},
		{"bad side", []Entry{{"a", 0, 5, "XAF"}, c("b", 5)}, ErrInvalidEntry},
		{"mixed currency does not net", []Entry{d("a", 5), {"b", Credit, 5, "EUR"}}, ErrUnbalanced},
		{"overflow", []Entry{d("a", math.MaxInt64), d("b", 1), c("c", 1)}, ErrAmountOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.entries); !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func FuzzValidateNeverPanics(f *testing.F) {
	f.Add(int64(100), int64(100))
	f.Fuzz(func(_ *testing.T, a, b int64) {
		_ = Validate([]Entry{{"x", Debit, a, "XAF"}, {"y", Credit, b, "XAF"}})
	})
}
