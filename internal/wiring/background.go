package wiring

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/zellapay/zellapay/internal/operators/simulator"
)

const (
	tickInterval  = time.Second
	sweepBatch    = 100
	sweepInterval = 2 * time.Second
	webhookBatch  = 100
)

// Run drives all background work until ctx ends: the simulator's callbacks
// (sandbox only), sweeping open collections, publishing outbox events and
// delivering webhooks. Every step is idempotent and safe to run from several
// processes at once. It is run by the worker, and by the api process itself
// when the in-memory simulator is enabled.
func (c *Container) Run(ctx context.Context, log *slog.Logger) {
	tick := time.NewTicker(tickInterval)
	defer tick.Stop()
	var lastSweep time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			if c.Simulator != nil {
				c.deliverSimulatorCallbacks(ctx, log)
			}
			if now.Sub(lastSweep) >= sweepInterval {
				c.sweep(ctx, log)
				c.sweepPayouts(ctx, log)
				c.settle(ctx, log)
				lastSweep = now
			}
			c.runWebhooks(ctx, log)
		}
	}
}

func (c *Container) runWebhooks(ctx context.Context, log *slog.Logger) {
	if _, err := c.Webhooks.Dispatch(ctx, webhookBatch); err != nil {
		log.Error("webhook dispatch failed", "err", err)
	}
	res, err := c.Webhooks.DeliverDue(ctx, webhookBatch)
	if err != nil {
		log.Error("webhook delivery failed", "err", err)
		return
	}
	if res.Attempted > 0 {
		log.Info("webhooks", "sent", res.Attempted, "ok", res.Succeeded,
			"retrying", res.Retrying, "dead", res.Dead)
	}
}

func (c *Container) deliverSimulatorCallbacks(ctx context.Context, log *slog.Logger) {
	for _, cb := range c.Simulator.DueCallbacks() {
		req, err := http.NewRequestWithContext(ctx,
			http.MethodPost, "/callbacks/simulator", bytes.NewReader(cb.Body))
		if err != nil {
			log.Error("sandbox callback request", "err", err)
			continue
		}
		req.Header.Set(simulator.SignatureHeader, cb.Signature)
		if err := c.Callbacks.Handle(ctx, simulator.Code, req); err != nil {
			log.Error("sandbox callback failed", "err", err)
		}
	}
}

// Sweep runs one pass over due collections.
func (c *Container) sweep(ctx context.Context, log *slog.Logger) {
	res, err := c.Collections.Sweep(ctx, sweepBatch)
	if err != nil {
		log.Error("sweep failed", "err", err)
		return
	}
	if res.Checked > 0 {
		log.Info("sweep", "checked", res.Checked, "changed", res.Changed, "failed", res.Failed)
	}
}

func (c *Container) sweepPayouts(ctx context.Context, log *slog.Logger) {
	res, err := c.Payouts.Sweep(ctx, sweepBatch)
	if err != nil {
		log.Error("payout sweep failed", "err", err)
		return
	}
	if res.Checked > 0 {
		log.Info("payout sweep",
			"checked", res.Checked, "changed", res.Changed, "failed", res.Failed)
	}
}

// settle releases collected funds whose settlement delay has passed.
func (c *Container) settle(ctx context.Context, log *slog.Logger) {
	res, err := c.Collections.Settle(ctx, sweepBatch)
	if err != nil {
		log.Error("settlement failed", "err", err)
		return
	}
	if res.Settled > 0 || res.Failed > 0 {
		log.Info("settlement", "settled", res.Settled, "failed", res.Failed)
	}
}
