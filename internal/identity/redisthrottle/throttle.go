// Package redisthrottle slows down password and code guessing, using Redis
// so every API instance shares the same counters.
package redisthrottle

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/clock"
	"github.com/zellapay/zellapay/internal/platform/crypto"
)

// Policy sets the limits. DefaultPolicy is the design's.
type Policy struct {
	// LockAfter is the number of failures on one account that starts delays.
	LockAfter int
	// Delays are the waits applied to the failures after LockAfter, in turn;
	// the last one repeats. The account is never disabled, so an attacker
	// cannot lock a real user out for long.
	Delays []time.Duration
	// FailureMemory is how long a failure count is remembered.
	FailureMemory time.Duration
	// IPWindow, IPMaxFailures and IPMaxAccounts limit one source address:
	// failures, and distinct accounts tried, inside a sliding window.
	IPWindow      time.Duration
	IPMaxFailures int
	IPMaxAccounts int
}

// DefaultPolicy returns the limits from the design.
func DefaultPolicy() Policy {
	return Policy{
		LockAfter: 5,
		Delays: []time.Duration{
			time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 15 * time.Minute,
		},
		FailureMemory: 24 * time.Hour,
		IPWindow:      10 * time.Minute, IPMaxFailures: 20, IPMaxAccounts: 10,
	}
}

// Limiter implements identity.Throttle on Redis.
type Limiter struct {
	rdb    redis.Cmdable
	index  *crypto.BlindIndexer
	clock  clock.Clock
	policy Policy
}

var _ identity.Throttle = (*Limiter)(nil)

// New returns a Limiter. index keys the hashes of accounts and addresses, so
// Redis never holds an email or an IP address in the clear.
func New(
	rdb redis.Cmdable, index *crypto.BlindIndexer, c clock.Clock, p Policy,
) *Limiter {
	return &Limiter{rdb: rdb, index: index, clock: c, policy: p}
}

func (l *Limiter) id(label, value string) string {
	return hex.EncodeToString(l.index.Index("throttle:"+label, value)[:16])
}

type keys struct{ failures, lock, ip, ipAccounts string }

func (l *Limiter) keys(a identity.Attempt) keys {
	acct := l.id("account:"+string(a.Realm), a.Account)
	k := keys{
		failures: fmt.Sprintf("thr:%s:fail:%s", a.Realm, acct),
		lock:     fmt.Sprintf("thr:%s:lock:%s", a.Realm, acct),
	}
	if a.IP != "" {
		ip := l.id("ip:"+string(a.Realm), a.IP)
		k.ip = fmt.Sprintf("thr:%s:ip:%s", a.Realm, ip)
		k.ipAccounts = fmt.Sprintf("thr:%s:ipacct:%s", a.Realm, ip)
	}
	return k
}

// Wait returns how long the attempt must wait, or zero.
func (l *Limiter) Wait(ctx context.Context, a identity.Attempt) (time.Duration, error) {
	k := l.keys(a)
	now := l.clock.Now()
	locked, err := l.lockRemaining(ctx, k.lock, now)
	if err != nil {
		return 0, err
	}
	if k.ip == "" {
		return locked, nil
	}
	ipWait, err := l.ipWait(ctx, k, now)
	if err != nil {
		return 0, err
	}
	return max(locked, ipWait), nil
}

func (l *Limiter) lockRemaining(
	ctx context.Context, key string, now time.Time,
) (time.Duration, error) {
	raw, err := l.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read lock: %w", err)
	}
	until, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse lock: %w", err)
	}
	return max(0, time.UnixMilli(until).Sub(now)), nil
}

// Succeeded forgets the account's failures. The address's window is kept: a
// correct password does not excuse the guesses made before it.
func (l *Limiter) Succeeded(ctx context.Context, a identity.Attempt) error {
	k := l.keys(a)
	if err := l.rdb.Del(ctx, k.failures, k.lock).Err(); err != nil {
		return fmt.Errorf("clear failures: %w", err)
	}
	return nil
}
