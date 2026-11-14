# Data, security, observability and testing

## 12. Data

PostgreSQL 17. One database in phase 1, with one schema per module.

- Money: `bigint`, plus `currency char(3)`.
- IDs: prefixed ULIDs (`col_`, `pay_`, `mer_`).
- Migrations: `goose`, forward-only, reviewed.
- Roles: separate least-privilege roles per binary. The `api` role cannot rewrite ledger entries.
- Queries: `sqlc` and `pgx`.
- Job queue: `river` on Postgres, so job enqueue and business writes share a transaction.
- Redis: rate limits, idempotency fast path, short locks. Never the source of truth.
- Backups: point-in-time recovery, restore tested on a schedule.

## 13. Security

- TLS everywhere. mTLS between internal services. Secrets in a secret manager.
- API keys hashed. Webhook secrets encrypted at rest.
- Operator credentials only in `operator-gateway`.
- Audit log for every admin and merchant-setting change, append-only.
- Dashboard: MFA required, session or EdDSA JWT, role-based access.
- PII minimised and encrypted at rest where stored (phone numbers, KYC documents in object storage with restricted access).
- Dependencies scanned with `govulncheck` in CI. Container images are distroless.

## 14. Observability and operations

- OpenTelemetry traces across HTTP, gRPC, jobs and webhooks. Prometheus metrics. Structured logs with request IDs, no secrets or full phone numbers.
- SLOs to start with: collection initiation p99 latency, collection success rate per operator, webhook delivery lag, `AMBIGUOUS` count and age.
- Alerts on stuck transactions, ledger invariant failure, operator error rate, and reconciliation breaks.
- Runbooks for each alert.

## 17. Testing strategy

- Unit tests for state machines and ledger posting rules (table-driven).
- Property tests: random sequences of operations never break "entries sum to zero" and "no negative available balance".
- Integration tests on testcontainers (Postgres, Redis) against the simulator.
- Concurrency tests with `-race` for transfers and holds.
- Fuzz tests for request parsing and callback parsing.
- Load tests with k6. Chaos tests that inject operator timeouts, duplicate callbacks and database failover, then assert the books still balance.
