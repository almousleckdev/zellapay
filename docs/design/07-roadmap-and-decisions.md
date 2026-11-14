# Roadmap, ADRs and open questions

## 18. Roadmap

| Phase | Deliverable |
|---|---|
| 0 | Repo, CI, docs skeleton, first ADRs, platform package |
| 1 | Ledger, merchants, API keys, idempotency, simulator adapter |
| 2 | Collections end to end with webhooks and sandbox |
| 3 | Payouts with holds, batches and risk rules |
| 4 | `AMBIGUOUS` resolution, reconciliation, settlement and statements |
| 5 | Dashboard and admin console, OpenAPI SDKs, developer docs |
| 6 | First real operator adapter (as soon as access exists), security review, pilot merchants |
| 7 | Second operator, failover, payment links and QR, hardening |

## 19. Initial ADRs to write

1. Go as the implementation language.
2. Modular monolith with three deployables.
3. PostgreSQL as the system of record, with a Postgres-backed job queue.
4. Double-entry ledger with `int64` amounts.
5. API keys over rotating secrets for merchant authentication.
6. Signed webhooks, at-least-once with replay.
7. Simulator-first operator development.

## 20. Open questions

- Which operator statement formats and delivery channels will Airtel and Moov provide?
- Settlement model: do we hold merchant funds in our own float account, or does each operator settle directly to merchants?
- Required KYC level per merchant tier, and who performs it.
- Hosting region and data residency requirements.
- Fee model: percentage, flat, or tiered, and who bears operator fees.
- Refund and chargeback rules for mobile money.
- Regulatory path and the reporting obligations that follow from it.
