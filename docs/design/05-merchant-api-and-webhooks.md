# Merchant API, webhooks and sandbox

## 8. Merchant API

REST over HTTPS, JSON, versioned in the path (`/v1`). OpenAPI 3.1 is the source of truth, and SDKs are generated from it.

### Authentication
- API keys: `zp_test_...` and `zp_live_...`, shown once, stored as hashes, scoped (`collections:write`, `payouts:write`, `read`), rotatable with an overlap window.
- Optional IP allowlist per key.
- Requests may be signed (HMAC) for high-value scopes.

### Idempotency
`Idempotency-Key` header is required on all POSTs. Same key and same body returns the original response. Same key and different body returns `409`. Keys expire after 24 hours. Stored in Postgres with a Redis fast path.

### Endpoints (v1)

| Method and path | Purpose |
|---|---|
| `POST /v1/collections` | Request a payment from a customer's phone |
| `GET /v1/collections/{id}` | Get a collection |
| `GET /v1/collections` | List with keyset pagination |
| `POST /v1/payouts` | Send money to a phone number |
| `POST /v1/payouts/batch` | Up to N payouts in one call |
| `GET /v1/payouts/{id}` | Get a payout |
| `POST /v1/refunds` | Refund a collection |
| `GET /v1/balance` | Available, pending, held |
| `GET /v1/statements` | Date-ranged statement export |
| `POST /v1/webhook-endpoints` | Register an endpoint |
| `POST /v1/webhook-events/{id}/replay` | Replay an event |
| `GET /v1/operators` | Operators and their current health |

Later: payment links, QR codes, split payments, customer lookup.

### Example

```
POST /v1/collections
Idempotency-Key: 7c1f...
{
  "amount": 10000,
  "currency": "XAF",
  "customer_phone": "+24107XXXXXXX",
  "merchant_reference": "ORDER-1042",
  "description": "Order 1042",
  "callback_url_id": "we_123"
}

201
{
  "id": "col_01J...",
  "status": "pending",
  "amount": 10000,
  "currency": "XAF",
  "operator": "airtel_money",
  "merchant_reference": "ORDER-1042",
  "created_at": "..."
}
```

Errors use one envelope: `{ "error": { "code", "message", "request_id", "details" } }` with stable machine-readable codes.

## 9. Webhooks

- Events: `collection.succeeded`, `collection.failed`, `collection.expired`, `payout.succeeded`, `payout.failed`, `refund.succeeded`, and so on, catalogued and versioned.
- Delivery: at-least-once, written to the outbox in the same transaction as the state change, then delivered by the worker.
- Signing: `ZellaPay-Signature: t=<unix>,v1=<hmac_sha256(secret, t + "." + body)>`. Receivers reject timestamps older than 5 minutes.
- Retries: exponential backoff for up to 72 hours, then dead-letter. Merchants can see the delivery log and replay from the dashboard or API.
- Every event has a stable `id`. Merchants dedupe on it.

## 15. Sandbox

- Separate `zp_test_` keys and a separate database.
- Test phone numbers trigger specific outcomes (success, failure, timeout, ambiguous, duplicate callback).
- Sandbox webhooks are signed exactly as in production.
- The simulator doubles as the integration test target.
