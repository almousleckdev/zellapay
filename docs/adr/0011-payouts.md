# ADR 0011: Payouts and settlement

Status: Accepted
Date: 2026-11-14

## Context
Money leaves ZellaPay when a merchant sends a payout. A mistake here is a
direct loss: paying twice, paying without the funds, or refunding a payout
that was paid. Payouts also need funds that are withdrawable, which collections
did not yet provide.

## Decision

### Funds are held before anything is sent
The amount plus our fee moves from the merchant's available balance to a hold
account, in one ledger journal. The ledger refuses the journal if the balance
is short, so the funds check is atomic with the reservation and needs no
separate read. A refused payout is stored as rejected and returns the same
error on every repeat.

### One resolution, enforced by the ledger
A payout ends in exactly one of two journals: **capture** (the hold is spent,
the operator balance falls, the fee is revenue) or **release** (the hold
returns to available). Both use the same idempotency key with different
content, so the ledger itself refuses the second one. A payout can therefore
never be both paid and refunded, whatever an operator later reports; such a
report is recorded as a contradiction for a person.

### The operator is asked at most once
The operator reference is the payout ID, so repeating the request cannot pay
twice. A timeout or unclear error is an unknown outcome: the payout becomes
ambiguous and the funds stay held until the operator's status answers. Only a
definite refusal, or an operator that has no record of a request we never
confirmed, returns the funds.

### Crashes are recoverable
The order is: store, hold, mark held, ask the operator, mark pending. A payout
stored but never held was never sent and is ended safely. One held but not
confirmed may have reached the operator, so it is resolved by asking the
operator by reference before anything is returned.

### Settlement
Collected funds sit in `pending` and move to `available` once the settlement
delay has passed (default 24 hours, `ZP_SETTLEMENT_DELAY`). The release is an
idempotent journal keyed on the collection. Without it, no merchant could ever
pay out.

### Limits and fees
A single payout may not exceed `ZP_PAYOUT_MAX_AMOUNT` (default 5,000,000).
Payouts have their own fee, `payout_fee_bps`, separate from the collection fee
and zero by default. The fee is fixed at creation.

### Batches
A batch of up to 100 payouts reports each item separately. Item idempotency
keys derive from the batch key, so repeating a batch never pays anyone twice,
even after a partial failure.

## Consequences
- Velocity limits, risk rules and review of large payouts are not built; the
  cap is the only control today.
- A payout stuck pending at the operator past 24 hours is flagged, never
  failed by us, and keeps its funds held.
- A held-for-review payout (amount mismatch) needs an admin tool that does not
  exist yet.
- Operators must treat a repeated reference as the same request and answer
  status queries by our reference; adapters are tested for both.
