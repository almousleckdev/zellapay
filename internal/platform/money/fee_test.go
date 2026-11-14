package money

import (
	"errors"
	"math"
	"testing"
)

func TestFee(t *testing.T) {
	cases := []struct {
		name   string
		amount int64
		bps    int
		want   int64
	}{
		{"zero rate", 10000, 0, 0},
		{"one and a half percent", 10000, 150, 150},
		{"rounds half up", 1000, 150, 15},
		{"rounds up at the half", 10, 150, 0},
		{"rounds up above the half", 400, 150, 6},
		{"below the half rounds down", 33, 150, 0},
		{"full rate", 5000, 10000, 5000},
		{"one unit", 1, 5000, 1},
	}
	for _, c := range cases {
		got, err := Fee(c.amount, c.bps)
		if err != nil || got != c.want {
			t.Errorf("%s: got %d, %v; want %d", c.name, got, err, c.want)
		}
	}
}

func TestFeeNeverExceedsTheAmount(t *testing.T) {
	for amount := int64(1); amount <= 300; amount++ {
		for _, bps := range []int{0, 1, 99, 150, 5000, 9999, 10000} {
			fee, err := Fee(amount, bps)
			if err != nil || fee < 0 || fee > amount {
				t.Fatalf("amount %d bps %d: fee %d, %v", amount, bps, fee, err)
			}
		}
	}
}

func TestFeeRejectsBadInput(t *testing.T) {
	for _, c := range []struct {
		amount int64
		bps    int
	}{{0, 100}, {-5, 100}, {100, -1}, {100, 10001}} {
		if _, err := Fee(c.amount, c.bps); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: got %v", c, err)
		}
	}
}

func TestFeeDetectsOverflow(t *testing.T) {
	if _, err := Fee(math.MaxInt64, 10000); !errors.Is(err, ErrOverflow) {
		t.Fatalf("got %v", err)
	}
}
