# Overview

## 1. Purpose

ZellaPay is a payment aggregator for Gabon. Merchants integrate once through one API and accept and send money across mobile money operators (Airtel Money, Moov Money first), with cards and banks later.

We compete with existing aggregators such as PVit. The goal is to win on reliability, security and developer experience, not on feature count.

### Goals
- One merchant API for collections (customer pays merchant) and payouts (merchant pays out to a phone number).
- A correct double-entry ledger as the source of truth for all money.
- Every write is idempotent. Every transaction has an explicit state machine.
- Reliable, signed webhooks with replay.
- `AMBIGUOUS` outcomes resolved automatically wherever possible.
- A sandbox that behaves like production, including failures.

### Non-goals (v1)
- Cards and bank rails (designed for, not built).
- Multi-country support (designed for, not built).
- Lending, wallets for end customers, FX.
- Microservices from day one (see section 3).

## 2. Constraints and assumptions

- No operator agreements yet. Airtel and Moov are in discussion. We build against a **simulator** first and write real adapters when access is granted.
- Operator APIs are poorly documented and unreliable. We assume timeouts, duplicate callbacks, missing callbacks and wrong statuses.
- Licensing and regulatory status in Gabon and CEMAC must be confirmed with counsel. This document does not cover it.
- Currency is XAF (no minor units in practice). We still store amounts as `int64` in the smallest unit and keep a per-currency exponent so other currencies can be added.
