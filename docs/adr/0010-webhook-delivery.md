# ADR 0010: Webhook delivery

Status: Accepted
Date: 2026-11-14

## Context
Merchants must learn the final result of a collection without polling. The
notification must not be lost, must be trustworthy, and must not become a way
to attack ZellaPay's own network.

## Decision

### Events leave through a transactional outbox
When a collection reaches a final status, an outbox row is written in the same
database transaction. A dispatcher claims unpublished rows with
`FOR UPDATE SKIP LOCKED`, creates one event per object (unique on type and
object) and one delivery per subscribed endpoint, and marks the row published.
A crash at any point repeats work that is idempotent. The module that owns the
outbox exposes it through an interface; the webhooks module never reads
another module's tables.

### Delivery is at least once
Deliveries are claimed with a lease, so two workers never send the same one,
and a worker that dies loses its claim when the lease expires. Failures retry
with exponential backoff (30 seconds doubling to six hours) for 72 hours, then
become dead. Every attempt is logged. Merchants deduplicate on the event ID.

### Signing
`HMAC-SHA256(secret, "<unix seconds>.<raw body>")`, carried in
`ZellaPay-Signature` as `t=...,v1=...`. Binding the timestamp to the body lets
receivers reject replays. Each attempt is signed afresh. Signing secrets are
random, shown once, and stored encrypted with a context that ties each secret
to its endpoint.

### Outbound requests are treated as hostile
Merchants choose the URLs, so the sender is a server-side request forgery
risk. URLs must be HTTPS with a public host. The destination address is
checked again at connect time, on the address actually dialled, to defeat
DNS rebinding. Redirects are never followed, proxies are not used, response
bodies are read only up to a small limit, and stored errors are reduced to
stable codes because raw network errors can name internal addresses.
Production cannot enable private destinations.

### The payload is the API's object
`data.object` is produced by the same function as the merchant API response,
so webhooks and polling never disagree.

## Consequences
- A new event type needs an outbox row type, a source mapping and a place in
  the OpenAPI contract and `docs/webhooks.md`.
- Deliveries to one slow endpoint can occupy sender capacity until their
  timeout. Per-endpoint concurrency limits and automatic disabling of
  persistently failing endpoints are not built yet.
- Retry timing has no jitter, so a recovering endpoint may see bursts.
- There is no delivery-log API for merchants yet; the log is in the database.
