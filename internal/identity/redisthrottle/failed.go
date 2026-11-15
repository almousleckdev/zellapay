package redisthrottle

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/zellapay/zellapay/internal/identity"
)

// Failed records a wrong answer against the account and the address.
func (l *Limiter) Failed(ctx context.Context, a identity.Attempt) error {
	k := l.keys(a)
	now := l.clock.Now()
	n, err := l.countFailure(ctx, k)
	if err != nil {
		return err
	}
	if delay := l.delayFor(n); delay > 0 {
		until := strconv.FormatInt(now.Add(delay).UnixMilli(), 10)
		err := l.rdb.Set(ctx, k.lock, until, delay+time.Minute).Err()
		if err != nil {
			return fmt.Errorf("set lock: %w", err)
		}
	}
	if k.ip == "" {
		return nil
	}
	return l.recordIP(ctx, k, a, now)
}

func (l *Limiter) countFailure(ctx context.Context, k keys) (int64, error) {
	pipe := l.rdb.TxPipeline()
	incr := pipe.Incr(ctx, k.failures)
	pipe.Expire(ctx, k.failures, l.policy.FailureMemory)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("count failure: %w", err)
	}
	return incr.Val(), nil
}

// delayFor returns the wait after the n-th failure, or zero before the
// account starts to be slowed down.
func (l *Limiter) delayFor(n int64) time.Duration {
	over := int(n) - l.policy.LockAfter
	if over < 0 || len(l.policy.Delays) == 0 {
		return 0
	}
	return l.policy.Delays[min(over, len(l.policy.Delays)-1)]
}

func (l *Limiter) recordIP(
	ctx context.Context, k keys, a identity.Attempt, now time.Time,
) error {
	score := float64(now.UnixMilli())
	member := strconv.FormatInt(now.UnixNano(), 10) + ":" + l.id("member", a.Account)
	pipe := l.rdb.TxPipeline()
	pipe.ZAdd(ctx, k.ip, redis.Z{Score: score, Member: member})
	pipe.ZAdd(ctx, k.ipAccounts, redis.Z{Score: score, Member: l.id("seen", a.Account)})
	pipe.Expire(ctx, k.ip, l.policy.IPWindow+time.Minute)
	pipe.Expire(ctx, k.ipAccounts, l.policy.IPWindow+time.Minute)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("record address failure: %w", err)
	}
	return nil
}
