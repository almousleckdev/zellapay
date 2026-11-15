package redisthrottle

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// ipWait returns how long a source address must wait, judged by the failures
// and distinct accounts it produced inside the sliding window.
func (l *Limiter) ipWait(ctx context.Context, k keys, now time.Time) (time.Duration, error) {
	cutoff := strconv.FormatInt(now.Add(-l.policy.IPWindow).UnixMilli(), 10)
	failures, err := l.windowWait(ctx, window{k.ip, l.policy.IPMaxFailures}, cutoff, now)
	if err != nil {
		return 0, err
	}
	accounts, err := l.windowWait(ctx, window{k.ipAccounts, l.policy.IPMaxAccounts}, cutoff, now)
	if err != nil {
		return 0, err
	}
	return max(failures, accounts), nil
}

// window is a sliding-window set and the number of entries it may hold.
type window struct {
	key   string
	limit int
}

// windowWait trims old entries, and when the set is at its limit returns how
// long until the oldest entry leaves the window.
func (l *Limiter) windowWait(
	ctx context.Context, w window, cutoff string, now time.Time,
) (time.Duration, error) {
	key, limit := w.key, w.limit
	if err := l.rdb.ZRemRangeByScore(ctx, key, "-inf", cutoff).Err(); err != nil {
		return 0, fmt.Errorf("trim window: %w", err)
	}
	n, err := l.rdb.ZCard(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("count window: %w", err)
	}
	if n < int64(limit) {
		return 0, nil
	}
	oldest, err := l.rdb.ZRangeWithScores(ctx, key, 0, 0).Result()
	if err != nil || len(oldest) == 0 {
		return 0, err
	}
	leaves := time.UnixMilli(int64(oldest[0].Score)).Add(l.policy.IPWindow)
	return max(time.Second, leaves.Sub(now)), nil
}
