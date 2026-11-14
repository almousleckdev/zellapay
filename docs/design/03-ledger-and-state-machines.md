# Ledger and state machines

## 5. Ledger

Double-entry, append-only. Amounts are `int64`. Every journal entry sums to zero per currency.

### Chart of accounts (v1)

| Account | Type | Meaning |
|---|---|---|
| `operator:airtel:settlement` | asset | Money Airtel owes us |
| `operator:moov:settlement` | asset | Money Moov owes us |
| `merchant:{id}:available` | liability | Merchant balance we owe, withdrawable |
| `merchant:{id}:pending` | liability | Received but not yet settled or released |
| `merchant:{id}:hold` | liability | Reserved for in-flight payouts |
| `zella:fees` | revenue | Our fee income |
| `operator:{op}:fees` | expense | Fees charged by operators |
| `zella:suspense` | asset | Unmatched or `AMBIGUOUS` amounts |

### Example: successful collection of 10,000 XAF, fee 150

| Account | Debit | Credit |
|---|---|---|
| `operator:airtel:settlement` | 10,000 | |
| `merchant:m1:pending` | | 9,850 |
| `zella:fees` | | 150 |

Rules:
- No update or delete on entries. Corrections are reversing entries.
- Balances are derived from entries. A cached balance is allowed but checked by a nightly invariant job.
- Row-level locks with consistent lock ordering for any operation touching more than one account.

## 6. Transaction state machines

### Collection

```
CREATED ─▶ PENDING ─▶ SUCCEEDED
              │
              ├─▶ FAILED
              ├─▶ EXPIRED
              └─▶ AMBIGUOUS ─▶ SUCCEEDED | FAILED  (via resolution)
```

- `CREATED`: validated, idempotency key stored, risk passed. Nothing sent yet.
- `PENDING`: request accepted by operator, waiting for the customer to approve on their phone.
- `AMBIGUOUS`: operator response lost, timed out or contradictory. Funds are never assumed either way.
- `SUCCEEDED` posts ledger entries. `FAILED` and `EXPIRED` post none.
- Terminal states never change. A late `SUCCEEDED` after `FAILED` raises a break for manual review, not an automatic flip.

### Payout

```
CREATED ─▶ FUNDS_HELD ─▶ SENT ─▶ SUCCEEDED
              │            │
              │            ├─▶ FAILED ─▶ (hold released)
              │            └─▶ AMBIGUOUS ─▶ SUCCEEDED | FAILED
              └─▶ REJECTED (risk or insufficient funds)
```

Funds are held in the ledger before anything is sent. On `FAILED` the hold is released. On `AMBIGUOUS` the hold stays until resolved.

### `AMBIGUOUS` resolution

1. Immediate: query operator status using our reference.
2. Retry with exponential backoff for a bounded window.
3. If still unknown, wait for the operator's daily statement and match on reference and amount.
4. If still unmatched after the window, open a support case and keep funds in `zella:suspense` / hold.

Every step is recorded as an event on the transaction.
