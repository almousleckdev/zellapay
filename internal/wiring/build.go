package wiring

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/zellapay/zellapay/internal/callbacks"
	"github.com/zellapay/zellapay/internal/collections"
	collectionstore "github.com/zellapay/zellapay/internal/collections/pgstore"
	ledgerstore "github.com/zellapay/zellapay/internal/ledger/pgstore"
	"github.com/zellapay/zellapay/internal/merchants"
	merchantstore "github.com/zellapay/zellapay/internal/merchants/pgstore"
	"github.com/zellapay/zellapay/internal/operators"
	"github.com/zellapay/zellapay/internal/operators/router"
	"github.com/zellapay/zellapay/internal/operators/simulator"
	"github.com/zellapay/zellapay/internal/payouts"
	payoutstore "github.com/zellapay/zellapay/internal/payouts/pgstore"
	"github.com/zellapay/zellapay/internal/platform/clock"
	"github.com/zellapay/zellapay/internal/platform/crypto"
	"github.com/zellapay/zellapay/internal/platform/idempotency"
	idemstore "github.com/zellapay/zellapay/internal/platform/idempotency/pgstore"
	"github.com/zellapay/zellapay/internal/platform/idempotency/rediscache"
	"github.com/zellapay/zellapay/internal/webhooks"
	"github.com/zellapay/zellapay/internal/webhooks/httpsender"
	webhookstore "github.com/zellapay/zellapay/internal/webhooks/pgstore"
)

const (
	idempotencyTTL     = 24 * time.Hour
	idempotencyLockTTL = time.Minute
	cacheMargin        = time.Hour // cache entries expire before the store's
	webhookTimeout     = 10 * time.Second
)

// Container holds the built services and the resources they share.
type Container struct {
	Settings    Settings
	Clock       clock.Clock
	Pool        *pgxpool.Pool
	Redis       *redis.Client
	Merchants   *merchants.Service
	Collections *collections.Service
	Payouts     *payouts.Service
	Webhooks    *webhooks.Service
	Callbacks   *callbacks.Dispatcher
	Idempotency idempotency.Store
	// Simulator is set only when enabled. It keeps state in memory, so it is
	// usable from the process that built it, not from a separate worker.
	Simulator *simulator.Simulator
}

// Build connects to the datastores and assembles every service.
func Build(ctx context.Context, s Settings, log *slog.Logger) (*Container, error) {
	return BuildWithClock(ctx, s, clock.System{}, log)
}

// BuildWithClock is Build with an explicit clock, for tests.
func BuildWithClock(
	ctx context.Context, s Settings, clk clock.Clock, log *slog.Logger,
) (*Container, error) {
	if err := s.Validate(); err != nil {
		return nil, fmt.Errorf("invalid settings: %w", err)
	}
	pool, err := pgxpool.New(ctx, s.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}
	opt, err := redis.ParseURL(s.RedisURL)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("ZP_REDIS_URL: not a valid redis URL")
	}
	c := &Container{Settings: s, Clock: clk, Pool: pool, Redis: redis.NewClient(opt)}
	if err := c.assemble(log); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// Close releases the datastore connections.
func (c *Container) Close() {
	c.Pool.Close()
	_ = c.Redis.Close()
}

// Ready reports whether the datastores answer.
func (c *Container) Ready(ctx context.Context) error {
	if err := c.Pool.Ping(ctx); err != nil {
		return fmt.Errorf("database: %w", err)
	}
	if err := c.Redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	return nil
}

func (c *Container) assemble(log *slog.Logger) error {
	sealer, err := crypto.NewSealer(
		[]crypto.Key{{ID: c.Settings.DataKeyID, Bytes: c.Settings.DataKey}}, c.Settings.DataKeyID)
	if err != nil {
		return fmt.Errorf("ZP_DATA_KEY: %w", err)
	}
	indexer, err := crypto.NewBlindIndexer(c.Settings.BlindIndexKey)
	if err != nil {
		return fmt.Errorf("ZP_BLIND_INDEX_KEY: %w", err)
	}
	rt, err := c.buildRouter()
	if err != nil {
		return err
	}
	c.Merchants = merchants.NewService(merchantstore.New(c.Pool), c.Clock, c.Settings.Mode)
	led := ledgerstore.New(c.Pool)
	collectionCfg := collections.DefaultConfig()
	collectionCfg.SettlementDelay = c.Settings.SettlementDelay
	c.Collections = collections.NewService(collections.Deps{
		Repo:   collectionstore.New(c.Pool, sealer, indexer),
		Terms:  c.Merchants,
		Router: rt, Ledger: led, Accounts: led, Clock: c.Clock, Log: log,
	}, collectionCfg)
	payoutCfg := payouts.DefaultConfig()
	payoutCfg.MaxAmount = c.Settings.PayoutMaxAmount
	c.Payouts = payouts.NewService(payouts.Deps{
		Repo:   payoutstore.New(c.Pool, sealer, indexer),
		Terms:  c.Merchants,
		Router: rt, Ledger: led, Accounts: led, Clock: c.Clock, Log: log,
	}, payoutCfg)
	c.Callbacks = callbacks.New(rt, c.Collections, c.Payouts)
	c.Webhooks = c.buildWebhooks(sealer, log)
	c.Idempotency = c.buildIdempotency(log)
	return nil
}

func (c *Container) buildWebhooks(sealer *crypto.Sealer, log *slog.Logger) *webhooks.Service {
	cfg := webhooks.DefaultConfig()
	cfg.Policy = c.Settings.WebhookPolicy
	return webhooks.NewService(webhooks.Deps{
		Repo:   webhookstore.New(c.Pool),
		Sender: httpsender.New(cfg.Policy, webhookTimeout),
		Streams: []webhooks.Stream{
			{
				Feed:   collectionFeed{svc: c.Collections},
				Source: collectionSource{svc: c.Collections},
			},
			{Feed: payoutFeed{svc: c.Payouts}, Source: payoutSource{svc: c.Payouts}},
		},
		Box: sealer, Clock: c.Clock, Log: log,
	}, cfg)
}

func (c *Container) buildIdempotency(log *slog.Logger) idempotency.Store {
	pg := idemstore.New(c.Pool, idemstore.Options{TTL: idempotencyTTL, LockTTL: idempotencyLockTTL})
	return idempotency.NewCachedStore(pg, rediscache.New(c.Redis), idempotencyTTL-cacheMargin, log)
}

// buildRouter registers the operators. Real adapters will be added here
// when operator access exists; until then production has none and refuses
// to start.
func (c *Container) buildRouter() (*router.Router, error) {
	var adapters []operators.Adapter
	var rules []router.Rule
	if c.Settings.SimulatorEnabled {
		c.Simulator = simulator.New(simulator.Config{Secret: c.Settings.SimulatorSecret}, c.Clock)
		adapters = append(adapters, c.Simulator)
		rules = append(rules, router.Rule{
			Prefix: c.Settings.SimulatorPrefix, Operator: simulator.Code,
		})
	}
	if len(adapters) == 0 {
		return nil, fmt.Errorf("no operator is configured")
	}
	return router.New(rules, adapters...)
}
