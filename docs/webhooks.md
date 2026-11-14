# Webhooks

ZellaPay sends an HTTP `POST` to your endpoint when a collection reaches a
final result. Use webhooks instead of polling.

## Events

| Type | Sent when |
|---|---|
| `collection.succeeded` | The customer paid and the money is recorded |
| `collection.failed` | The customer declined, the operator refused, or the request never arrived |
| `collection.expired` | The operator gave up waiting for the customer |
| `payout.succeeded` | The money reached the recipient and your held funds were spent |
| `payout.failed` | The payout did not happen and the held funds were returned to you |

Payout events carry the payout object, exactly as `GET /v1/payouts/{id}` returns
it.

## Body

```json
{
  "id": "evt_0f1e2d3c4b5a69788796a5b4",
  "type": "collection.succeeded",
  "created_at": "2026-11-14T12:00:00Z",
  "data": { "object": { "id": "col_...", "status": "succeeded", "amount": 10000 } }
}
```

`data.object` is exactly what `GET /v1/collections/{id}` returns.

## Headers

Header names are case-insensitive. Read them that way.

| Header | Meaning |
|---|---|
| `ZellaPay-Signature` | `t=<unix seconds>,v1=<hex HMAC-SHA256>` |
| `ZellaPay-Event-Id` | The event's `id`, for deduplication |
| `ZellaPay-Event-Type` | The event's `type` |
| `ZellaPay-Delivery-Attempt` | 1 for the first try, 2 for the first retry, and so on |

## Verify every request

Anyone who knows your URL can send you a request. Only ZellaPay can sign one.

1. Take the **raw request body**, before any JSON parsing.
2. Read `t` and `v1` from `ZellaPay-Signature`.
3. Compute `HMAC-SHA256(secret, t + "." + body)` as hex, using your endpoint's
   signing secret (`whsec_...`) as the key.
4. Compare with `v1` **in constant time**.
5. Reject the request if `t` is more than five minutes from your clock. This
   stops an attacker replaying a captured request.

```python
import hmac, hashlib, time

def verify(secret: str, header: str, body: bytes, tolerance: int = 300) -> bool:
    parts = dict(p.split("=", 1) for p in header.split(","))
    signed = parts["t"].encode() + b"." + body
    expected = hmac.new(secret.encode(), signed, hashlib.sha256).hexdigest()
    fresh = abs(time.time() - int(parts["t"])) <= tolerance
    return fresh and hmac.compare_digest(expected, parts["v1"])
```

The signing secret is shown **once**, when you create the endpoint, and is
stored encrypted. If you lose it, create a new endpoint.

## Delivery

- **Respond with any `2xx` within 10 seconds.** Do the real work afterwards.
  Anything else, including a timeout, counts as a failure.
- **Retries:** after a failure we wait 30 seconds, then 1, 2, 4 minutes and so
  on, doubling up to six hours between tries, for up to 72 hours. After that
  the delivery is dead; you can still replay it.
- **At least once.** The same event can arrive more than once. Deduplicate on
  `id`. Events for different collections can arrive in any order.
- **Redirects are not followed.** Give us the final URL.
- **Destinations:** HTTPS to a public address only. Private, loopback and
  link-local addresses are refused, and checked again when we connect.

## Replay

`POST /v1/webhook-events/{event_id}/replay` sends an event again to your
current endpoints, signed with a fresh timestamp. Use it after an outage on
your side.

## Limits

Up to 10 endpoints per merchant. Each subscribes to the event types it lists.
