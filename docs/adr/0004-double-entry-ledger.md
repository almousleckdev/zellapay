# ADR 0004: Double-entry ledger with int64 amounts

Status: Accepted
Date: 2026-11-14

## Context
Money correctness is the core risk of the product.

## Decision
Store amounts as int64 in the smallest unit with a currency code. Entries are append-only and every journal balances per currency. Corrections are reversing entries.

## Consequences
Auditable and testable by invariant. Requires discipline: no direct balance updates.
