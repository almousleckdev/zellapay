# Ledger rules

The ledger is the source of truth for all money. Mistakes here are
financial losses. Apply the root `CLAUDE.md` plus the rules below.

## Invariants (never violate, always test)

1. Every journal balances per currency: debits equal credits.
2. Entries are append-only. No update, no delete, enforced by database
   permissions and a trigger, not only by code.
3. A balance is derived from entries. A cached balance is verified by a
   scheduled invariant job and never trusted over entries.
4. Available balance never goes negative for merchant accounts.
5. Posting the same idempotency key twice yields one posting.
6. Amounts are positive `int64`. Direction is carried by the side.

## Rules

- This package imports no other domain module and no HTTP package.
- Posting is one database transaction. Lock accounts in sorted order.
- Holds are ledger postings, not flags. Release and capture post entries.
- Accounts are created explicitly with a type and currency. A posting to an
  unknown account fails. It never creates one.
- Every entry stores who or what caused it: transaction ID and event ID.
- Reversals reference the entry they reverse and cannot be reversed twice.
- No business policy (fees, limits, risk) lives here. Callers pass finished
  entries and the ledger validates and stores them.

## Tests required for any change

- Table tests for each posting shape.
- Property test: random journals keep all invariants.
- Concurrency test with `-race`: parallel transfers keep totals constant.
- Idempotency test: replaying a posting does not change balances.
