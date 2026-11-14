# ZellaPay

Payment aggregator for Gabon. One API to collect and pay out money across
mobile money operators, backed by a double-entry ledger.

[![CI](https://img.shields.io/badge/ci-github%20actions-2088ff)](.github/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.27-00add8)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)

> **Status:** Phase 1 in progress (ledger, merchants, idempotency, simulator).
> See the [roadmap](docs/design/07-roadmap-and-decisions.md).

## Overview

ZellaPay lets merchants accept payments from, and send money to, customers on
local mobile money networks through a single REST API.

| Capability | Description |
|---|---|
| **Collections** | Request a payment on a customer's phone and get a signed webhook |
| **Payouts** | Send money to a phone number, singly or in batches |
| **Refunds** | Reverse a collection through the ledger |
| **Ledger** | Append-only double-entry books, balanced per currency |
| **Webhooks** | At-least-once delivery, HMAC signatures, replay, delivery log |
| **Reconciliation** | Daily matching against operator statements |
| **Sandbox** | Test keys and phone numbers that simulate every outcome |

Operators: Airtel Money and Moov Money first. Cards and banks are designed for.

## Principles

- **Money is exact.** `int64` minor units, never floats. Every journal balances.
- **Every write is idempotent.** Retrying never double-charges.
- **Unknown is not failed.** A timeout never releases or re-sends funds.
- **Secure by default.** Encrypted at rest, signed in flight, least privilege.
- **Small and explicit.** Files under 300 lines, one responsibility each.

## Architecture

A modular monolith deployed as three binaries. Module boundaries are enforced
in CI so a module can be extracted into a service when there is a reason.

```
 Merchant backends / dashboard
            │ HTTPS
            ▼
   ┌──────────────────┐        ┌──────────────────┐
   │       api        │───────▶│    PostgreSQL    │
   └──────────────────┘        └──────────────────┘
            │ gRPC + mTLS               ▲
            ▼                           │
   ┌──────────────────┐        ┌──────────────────┐
   │ operator-gateway │◀───────│      worker      │
   └──────────────────┘        └──────────────────┘
            │                           │ signed webhooks
            ▼                           ▼
   Airtel / Moov / simulator     Merchant endpoints
```

| Binary | Responsibility |
|---|---|
| `api` | Merchant API, dashboard and admin backends, domain modules |
| `worker` | Polling, `AMBIGUOUS` resolution, payouts, webhook delivery, jobs |
| `operator-gateway` | Operator adapters, isolated credentials and allowlisting |

Domain modules: `ledger`, `collections`, `payouts`, `merchants`, `webhooks`,
`risk`, `reconciliation`, `reporting`, `operators`, `admin`.

Clean architecture: transport depends on application, which depends on the
domain. The domain imports no framework, database driver or HTTP package.
Full detail in [docs/design](docs/design).

## Technology

| Layer | Choice |
|---|---|
| Language | Go |
| HTTP | `net/http` with `chi` |
| Internal RPC | gRPC with mTLS |
| Database | PostgreSQL 17, `sqlc`, `pgx`, `goose` |
| Jobs | `river` on PostgreSQL |
| Cache and limits | Redis-compatible (Tair on Alibaba Cloud) |
| Observability | OpenTelemetry, Prometheus, Grafana, structured `slog` |
| Frontends | Next.js and TypeScript, generated SDKs from OpenAPI |
| Infrastructure | Terraform, Docker (distroless), Kubernetes, Helm |

## Cloud: Alibaba Cloud

| Concern | Service |
|---|---|
| Compute | Container Service for Kubernetes (ACK) |
| Images | Container Registry (ACR) with image scanning |
| Database | ApsaraDB RDS for PostgreSQL, high availability, point-in-time recovery |
| Cache | Tair (Redis compatible) |
| Edge | Application Load Balancer, Web Application Firewall, Anti-DDoS |
| Network | VPC with private subnets, NAT Gateway, security groups |
| Keys and secrets | Key Management Service (KMS) for envelope keys and secrets |
| Object storage | OSS with server-side encryption for statements and KYC files |
| Logs and metrics | Log Service (SLS), Managed Service for Prometheus, ARMS |
| Identity | RAM roles with least privilege, RRSA for pod identity |
| Audit | ActionTrail and Cloud Config |

The region is chosen against Gabon and CEMAC data residency requirements.

## DevOps

- **Infrastructure as code.** All cloud resources are defined in Terraform
  (`alicloud` provider) with remote state, review and drift detection.
- **Continuous integration.** Every pull request runs formatting, vet,
  `golangci-lint`, `gosec`, `govulncheck`, the file and line limit check, unit
  and integration tests with `-race`, and a build.
- **Continuous delivery.** Merges to `main` build a signed distroless image,
  push it to ACR and deploy to staging. Production promotion needs approval.
- **Progressive rollout.** Rolling updates with readiness probes, connection
  draining, automatic rollback on failed health or elevated error rate.
- **Environments.** `local`, `sandbox`, `staging`, `production`, each with
  separate databases, keys and cloud accounts or resource groups.
- **Database changes.** Forward-only migrations, rolling-deploy safe, run by a
  dedicated role in the pipeline.
- **Secrets.** Held in KMS, injected at runtime, never stored in git or images.
- **Operations.** SLOs, alerting, runbooks, blameless post-mortems, tested
  backups and restore drills.

## Security

- TLS 1.3 at the edge and mTLS between services.
- Envelope encryption (AES-256-GCM or XChaCha20-Poly1305) for sensitive data,
  with KMS-managed keys and rotation by key ID.
- Phone numbers encrypted, searched through keyed blind indexes.
- API keys stored as hashes, scoped, rotatable, optional IP allowlist.
- Webhooks signed with HMAC-SHA256 over timestamp and body, replay protected.
- Least-privilege database roles. Ledger history cannot be altered.
- Append-only audit log. MFA for dashboard and admin access.
- Dependency and image scanning in CI. No secrets or personal data in logs.

Report vulnerabilities privately to the security contact in `SECURITY.md`.

## Getting started

Requires Go 1.27 and Docker.

```bash
make up        # PostgreSQL (:5433), Redis (:6380) and MailDev (:1081), localhost only
make env       # creates .env.local with fresh secrets (once; never overwrites)
make migrate   # applies the database migrations
go run ./cmd/zellactl merchant create "My Shop" XAF
go run ./cmd/zellactl merchant activate <merchant-id>
go run ./cmd/zellactl merchant fee <merchant-id> 150          # collection fee, basis points
go run ./cmd/zellactl merchant payout-fee <merchant-id> 100   # payout fee, basis points
go run ./cmd/zellactl key issue <merchant-id> read,collections:write,payouts:write,webhooks:write
make run-api   # http://localhost:8080
```

To see email working, run `go run ./cmd/zellactl notify test you@example.test`
and open the MailDev inbox at <http://localhost:1081>.

Open `http://localhost:8080/docs/` for the interactive API reference and use
the issued key as the bearer token. The sandbox simulates the operator:
phone numbers ending in `00` succeed, other endings trigger failures and edge
cases (see [docs/simulator.md](docs/simulator.md)).

`make check` runs the limits, vet, race tests and linters.

## API example

```bash
curl http://localhost:8080/v1/collections \
  -H "Authorization: Bearer zp_test_..." \
  -H "Idempotency-Key: 7c1f0a52-..." \
  -d '{
    "amount": 10000,
    "currency": "XAF",
    "customer_phone": "+24107000000",
    "merchant_reference": "ORDER-1042"
  }'
```

The response is `pending`. The final state arrives as a signed
`collection.succeeded` or `collection.failed` webhook. The contract is
defined in `api/openapi.yaml`.

## Repository layout

| Path | Purpose |
|---|---|
| `cmd/` | Entry points: `api`, `worker`, `operator-gateway` |
| `internal/` | Domain modules and shared platform code |
| `api/` | OpenAPI specification |
| `db/` | Migrations and `sqlc` queries |
| `proto/` | gRPC definitions for the operator gateway |
| `sim/` | Operator simulator |
| `deploy/` | Docker, Terraform and Helm |
| `tests/` | Integration, invariant, load and chaos tests |
| `docs/` | Design, ADRs and runbooks |

## Documentation

- [Ecosystem map: how the four projects connect](docs/ecosystem/README.md)
- [Webhooks guide for merchants](docs/webhooks.md)
- [Design](docs/DESIGN.md)
- [Architecture decisions](docs/adr)
- [Engineering standards](CLAUDE.md)

## Contributing

Read [CLAUDE.md](CLAUDE.md) first. Files stay under 300 lines, changes are
small and tested, and `make check` must pass before review.

## License

[MIT](LICENSE)
