# ADR 0006: Signed webhooks with at-least-once delivery and replay

Status: Accepted
Date: 2026-11-14

## Context
Merchants must be able to trust and recover webhook events.

## Decision
Events are written to a transactional outbox, delivered at least once with exponential backoff, signed with HMAC-SHA256 over timestamp and body, and replayable.

## Consequences
Merchants must dedupe on event id. Delivery log storage grows and needs retention rules.
