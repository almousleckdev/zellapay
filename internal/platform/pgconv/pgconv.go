// Package pgconv converts between Go values and pgx types, in one place so
// every store converts the same way.
package pgconv

import (
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Timestamptz wraps t as a non-null timestamp.
func Timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// OptTimestamptz wraps t, or returns NULL when t is nil.
func OptTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return Timestamptz(*t)
}

// TimePtr returns the time, or nil for NULL.
func TimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

// Int32 narrows n to int32, saturating at the limits instead of wrapping.
func Int32(n int) int32 {
	switch {
	case n > math.MaxInt32:
		return math.MaxInt32
	case n < math.MinInt32:
		return math.MinInt32
	}
	return int32(n)
}
