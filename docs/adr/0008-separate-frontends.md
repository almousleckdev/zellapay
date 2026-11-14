# ADR 0008: Three separate frontends

Status: Accepted
Date: 2026-11-14

## Context
ZellaPay needs a public website, a merchant dashboard and an internal admin
console. They have different audiences, authentication and risk. The admin
console can suspend merchants and approve payouts.

## Decision
Build three independent Next.js and TypeScript projects, each in its own
repository, deployed and scoped separately:

| Project | Audience | Exposure |
|---|---|---|
| `zellaweb` | Public: marketing, pricing, sign-up, API docs | Public, mostly static |
| `zellamerchant` | Merchants and their staff | Public, login with MFA |
| `zellapayadmin` | ZellaPay staff | Restricted network, mandatory MFA |

All three call the same `api` binary through separate route groups with
separate authentication: `/v1` for merchant API keys, `/dashboard` for
merchant sessions and `/admin` for staff sessions. Clients are generated from
`api/openapi.yaml` so they cannot drift from the backend.

## Consequences
- A flaw in the public site cannot reach the admin console.
- Each project releases on its own schedule and can be locked down separately.
- Shared UI code needs a deliberate shared package or copy; that cost is
  accepted until duplication proves it is worth a package.
- Dashboard and admin authentication is a separate system from API keys and
  needs its own design before frontend work begins.

## Sharing code across repositories
- **API client:** not copied by hand. Each frontend generates a typed client
  from the backend's `api/openapi.yaml`, pinned to a released backend version,
  and CI fails if the committed client differs from the generated one.
- **UI components:** small pieces are duplicated at first. Extract a private
  shared package only when duplication causes real cost.
- **Standards:** each repository carries its own `CLAUDE.md`. The ecosystem
  map in `docs/ecosystem/` is the single place that describes how they relate.
