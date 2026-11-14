# Operators, risk and reconciliation

## 7. Operator adapter interface

```go
package operators

type Adapter interface {
    Code() string // "airtel_money", "moov_money", "simulator"

    InitiateCollection(ctx context.Context, r CollectionRequest) (OperatorRef, error)
    InitiatePayout(ctx context.Context, r PayoutRequest) (OperatorRef, error)
    Status(ctx context.Context, ref OperatorRef) (OperatorStatus, error)

    // Verify and parse an inbound operator callback.
    ParseCallback(ctx context.Context, req *http.Request) (CallbackEvent, error)

    Health(ctx context.Context) HealthReport
}

type OperatorStatus struct {
    State     State // Pending, Succeeded, Failed, Unknown
    RawCode   string
    RawBody   []byte // stored for audit
    FeeAmount *int64
}
```

Rules for adapters:
- All calls carry a deadline. No retries inside the adapter. Retry policy belongs to the caller.
- A timeout returns `ErrUnknownOutcome`, never `ErrFailed`.
- Raw request and response are stored (with secrets redacted) for every call.
- A **simulator adapter** ships first and supports scripted outcomes: success, failure, slow, duplicate callback, missing callback, wrong amount.

### Routing
Route by phone number prefix to the operator, with health-based failover only where a second route exists for the same operator group. Routing rules are data, not code.

## 10. Risk

- Per-merchant and per-customer limits (amount, count, velocity), configured as rules.
- Blocklists for phone numbers and merchants.
- Manual review queue for flagged payouts.
- Payout velocity caps and a new-merchant ramp.
- All decisions are logged with the rule that fired.

## 11. Reconciliation and settlement

- Daily import of operator statements (file or API per operator).
- Match on operator reference, amount and date. Output: matched, missing on our side, missing on theirs, amount mismatch.
- Breaks open a case. Nothing is auto-corrected without an audit entry.
- Settlement: merchant `pending` moves to `available` on a configurable schedule, with reports per merchant.
