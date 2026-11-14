# ADR 0009: Collection fees, settlement and operator authority

Status: Accepted
Date: 2026-11-14

## Context
A successful collection must be recorded in the ledger. That needs a fee model,
a rule for when funds become withdrawable, and clear rules for who decides the
outcome when the operator, our records and a callback disagree.

## Decision

### Fees
- Each merchant has `fee_bps`, a percentage in hundredths of a percent.
  `150` means 1.5 percent. The default is `0`, so nobody is charged by accident.
- The fee is computed when the collection is created, rounded half up in the
  smallest currency unit, and stored on the collection. Later changes to the
  merchant's rate never alter a collection already created.
- Fees are revenue in `zella:fees:<currency>`.

### Ledger posting on success
One journal, keyed `collection:<collection id>`:

| Account | Debit | Credit |
|---|---|---|
| `operator:<code>:settlement` | amount | |
| `merchant:<id>:pending` | | amount minus fee |
| `zella:fees:<currency>` | | fee |

The journal is posted before the status changes. A crash can leave money
recorded with the collection still open, which the next check repairs. It can
never leave a succeeded collection without its money.

### Settlement
Funds sit in the merchant's `pending` account. A scheduled job releases them
to `available` after a settlement delay, defaulting to one day (T+1) and
configurable per environment (`ZP_SETTLEMENT_DELAY`). It is built; see ADR 0011.

### Who decides the outcome
- The operator's status endpoint is authoritative. A callback is only a hint
  and is always confirmed by a status query before any money is recorded.
- A timeout or unclear error is never a failure. The collection becomes
  `ambiguous` and is resolved by querying the operator by our reference.
- We never expire a collection ourselves. Only an operator report moves it to
  `expired`. A collection still pending after 24 hours is flagged for
  attention and checked hourly.
- A callback whose amount differs from the request holds the collection for a
  person. Nothing is posted and no automatic check runs until someone decides.
- A final state never changes. A later contradicting report is recorded as an
  event for reconciliation and is not applied.

## Consequences
- Fee changes need an admin tool; until it exists the rate is set in the
  database or through the merchants service.
- The settlement release job and the held-collection resolution tool are
  required before real money flows.
- Operators must provide a status query by our reference and must treat a
  repeated reference as the same request. Adapters must pass the shared
  contract suite, which checks both.
