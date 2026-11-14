# Architecture

## 3. Architecture

### Style: modular monolith, three deployables

One Go module, strict package boundaries, three binaries.

| Binary | Role |
|---|---|
| `api` | Merchant API, dashboard and admin backends, all domain modules |
| `worker` | Background jobs: polling, `AMBIGUOUS` resolution, retries, payouts, reconciliation, settlement, webhook delivery (initially) |
| `operator-gateway` | Operator adapters. Isolated for credentials, IP allowlisting and fault containment |

`api` and `worker` share the same domain code and database. `operator-gateway` is called by them over gRPC with mTLS and deadlines.

### Why a monolith first
Distributed transactions between ledger and collections add failure modes we cannot afford before we have customers. Module boundaries (section 4) keep a later split cheap. The likely first extractions are `ledger` and the webhook dispatcher.

### Context diagram

```
Merchant backend ──HTTPS──▶ [ api ] ──▶ PostgreSQL
Merchant dashboard ─────────▶ [ api ] ──▶ Redis
Admin console ──────────────▶ [ api ]
                               │  gRPC (mTLS)
                               ▼
                     [ operator-gateway ] ──▶ Airtel / Moov / Simulator
                               ▲
[ worker ] ────────────────────┘
   │  └──▶ PostgreSQL (same DB, job queue + outbox)
   └──HTTPS──▶ Merchant webhook endpoints
Operators ──callbacks──▶ [ operator-gateway ] ──▶ [ api ] (internal)
```

## 4. Modules

Each module owns its tables and exposes a Go interface. Modules do not read each other's tables.

| Module | Responsibility |
|---|---|
| `ledger` | Accounts, journal entries, balances, holds. The only writer of ledger tables. |
| `collections` | Collection requests, state machine, operator orchestration |
| `payouts` | Payout requests, batches, state machine, funding checks |
| `merchants` | Onboarding, KYC status, API keys, operation accounts, settings |
| `webhooks` | Event catalog, endpoints, signing, delivery log, replay |
| `risk` | Limits, velocity rules, blocklists, manual review |
| `reconciliation` | Operator statement import, matching, break handling |
| `reporting` | Statements, exports, settlement reports |
| `operators` | Adapter interface, routing, health, failover |
| `admin` | Internal tooling, audit log |

Dependency rule: `collections` and `payouts` depend on `ledger`, `operators`, `risk`, `webhooks`. `ledger` depends on nothing in the domain. Enforce with `go-arch-lint` or an import check in CI.

## 16. Repository layout

```
zellapay/
  cmd/
    api/
    worker/
    operator-gateway/
  internal/
    ledger/ collections/ payouts/ merchants/ webhooks/
    risk/ reconciliation/ reporting/ operators/ admin/
    platform/        # config, logging, db, http, otel, idempotency
  api/openapi.yaml
  db/migrations/
  db/queries/
  proto/             # operator-gateway gRPC
  sim/               # operator simulator
  deploy/            # docker, terraform, helm later
  docs/
    DESIGN.md
    adr/
    runbooks/
  tests/             # integration, invariant, load, chaos
```
