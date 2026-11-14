# Integration contract

How the frontends and the backend agree on what is sent and returned.

## Source of truth

`api/openapi.yaml` (OpenAPI 3.1, split across files) describes the merchant
API under `/v1`. Contract tests in `api/` fail the build if an operation lacks
an `operationId`, a `401` response, or a required `Idempotency-Key` on a
`POST`.

Planned: `api/dashboard.openapi.yaml` and `api/admin.openapi.yaml` for the
other two route groups, with the same tests.

## Generated clients

Frontends do not hand-write API types. Each frontend generates a typed client
from the backend contract and commits the generated files under
`lib/api/generated/`. Generated files carry a `Code generated` header and are
exempt from the line limits.

- Generator: `openapi-typescript` with a small typed `fetch` wrapper
  (planned; install when the first screen needs it).
- Input: `../zellapay/api/openapi.yaml` in local development, a tagged
  backend release in CI.
- A CI step regenerates the client and fails if it differs from the
  committed files, so drift is caught.

## Versioning

- The URL carries the major version: `/v1`. Breaking changes mean `/v2`.
- Allowed without a new version: adding optional request fields, adding
  response fields, adding endpoints, adding enum values that clients must
  tolerate.
- Not allowed: removing or renaming fields, changing types or meanings,
  making optional fields required.
- Clients must ignore unknown response fields and must handle unknown enum
  values gracefully.

## Conventions every client must follow

| Topic | Rule |
|---|---|
| Amounts | Integers in the smallest unit (XAF has none). Never floats. Format for display only. |
| Currency | ISO 4217 code next to every amount. |
| Phone numbers | E.164. Mask when displaying: `+241 07 ** ** 42`. |
| Timestamps | UTC RFC 3339 on the wire. Convert to the user's zone for display. |
| IDs | Opaque prefixed strings. Never parse or construct them. |
| Idempotency | Send a fresh `Idempotency-Key` per user action and reuse it on retry. |
| Errors | One envelope: `{ error: { code, message, request_id } }`. Branch on `code`, never on `message`. Show `request_id` in support dialogs. |
| Pagination | Keyset: `limit`, `starting_after`, response `has_more`, `next_cursor`. |
| Statuses | Treat `pending` as unresolved. Final states are `succeeded`, `failed`, `expired`. |

## Webhooks

Frontends do not receive webhooks. They read current state from the API.
Webhook delivery is for merchants' own servers.

## Environments

| Environment | Backend base URL (env var) | Keys |
|---|---|---|
| Sandbox | `ZELLAPAY_API_URL` pointing at the sandbox | `zp_test_...` |
| Production | `ZELLAPAY_API_URL` pointing at production | `zp_live_...` |

Sandbox and production are separate deployments with separate databases. A
dashboard instance talks to exactly one of them. The merchant dashboard's
test/live switch is an open question (see `docs/design/07-roadmap-and-decisions.md`).

## Open contracts

- Dashboard and admin session login, refresh, logout and MFA.
- Role and permission model for merchant staff and ZellaPay staff.
- Audit log read endpoints for the admin console.
