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
	notifyBatch   = 50

	auditRoutineEvery = 5 * time.Minute
	auditFullEvery    = 24 * time.Hour
	auditBatch        = 1000
)

// schedule runs something at most once per interval.
type schedule struct {
	every time.Duration
	last  time.Time
}

func (s *schedule) due(now time.Time) bool {
	if now.Sub(s.last) < s.every {
		return false
	}
	s.last = now
	return true
}

// Run drives all background work until ctx ends: the simulator's callbacks
// (sandbox only), sweeping open collections, publishing outbox events,
// delivering webhooks and notifications, and checking the audit log. Every
// step is idempotent and safe to run from several processes at once. It is
// run by the worker, and by the api process itself when the in-memory
// simulator is enabled.
func (c *Container) Run(ctx context.Context, log *slog.Logger) {
	tick := time.NewTicker(tickInterval)
	defer tick.Stop()
	sweep := schedule{every: sweepInterval}
	routine := schedule{every: auditRoutineEvery}
	full := schedule{every: auditFullEvery}
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			if c.Simulator != nil {
				c.deliverSimulatorCallbacks(ctx, log)
			}
			if sweep.due(now) {
				c.sweep(ctx, log)
				c.sweepPayouts(ctx, log)
				c.settle(ctx, log)
			}
			c.runWebhooks(ctx, log)
			c.runNotifications(ctx, log)
			if routine.due(now) {
				c.checkAudit(ctx, log, false)
			}
			if full.due(now) {
				c.checkAudit(ctx, log, true)
			}
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

func (c *Container) runNotifications(ctx context.Context, log *slog.Logger) {
	res, err := c.Notify.DeliverDue(ctx, notifyBatch)
	if err != nil {
		log.Error("notification delivery failed", "err", err)
		return
	}
	if res.Attempted > 0 {
		log.Info("notifications", "sent", res.Sent, "retrying", res.Retrying, "dead", res.Dead)
	}
}

// checkAudit verifies the audit log. A break is logged as an error on every
// pass until a person resolves it: it means the history may have been altered.
func (c *Container) checkAudit(ctx context.Context, log *slog.Logger, full bool) {
	verify := c.Audit.Verify
	if full {
		verify = c.Audit.VerifyFull
	}
	res, err := verify(ctx, auditBatch)
	if err != nil {
		log.Error("audit verification could not run", "err", err)
		return
	}
	if res.Broken != nil {
		log.Error("AUDIT LOG TAMPERING DETECTED", "seq", res.Broken.Seq,
			"reason", res.Broken.Reason, "verified_through", res.VerifiedThrough)
	}
}
