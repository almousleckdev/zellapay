package pgconv

import (
	"math"
	"testing"
	"time"
)

func TestInt32SaturatesInsteadOfWrapping(t *testing.T) {
	cases := map[int]int32{
		0: 0, 42: 42, -7: -7,
		math.MaxInt32: math.MaxInt32, math.MaxInt32 + 1: math.MaxInt32,
		math.MinInt32: math.MinInt32, math.MinInt32 - 1: math.MinInt32,
		math.MaxInt64: math.MaxInt32,
	}
	for in, want := range cases {
		if got := Int32(in); got != want {
			t.Errorf("Int32(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestTimestampRoundTrip(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	if got := TimePtr(Timestamptz(now)); got == nil || !got.Equal(now) {
		t.Fatalf("got %v", got)
	}
	if TimePtr(OptTimestamptz(nil)) != nil {
		t.Fatal("nil must round-trip as NULL")
	}
	if got := TimePtr(OptTimestamptz(&now)); got == nil || !got.Equal(now) {
		t.Fatalf("got %v", got)
	}
}
