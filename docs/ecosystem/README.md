# ZellaPay ecosystem

ZellaPay is four projects in four repositories. This document is the map. If
you are an agent or engineer working in any one of them, read this first to
learn what the others are and where to find them.

Source of truth: this file lives in the backend repository (`zellapay`).
Frontend repositories link here and must not contradict it.

## The projects

| Project | Role | Audience | Stack | Dev port |
|---|---|---|---|---|
| `zellapay` | Backend: ledger, collections, payouts, merchants, webhooks, API | Everything else | Go, PostgreSQL, Redis | 8080 |
| `zellamerchant` | Merchant dashboard | Merchants and their staff | Next.js 16, TypeScript | 3001 |
| `zellapayadmin` | Internal admin console | ZellaPay staff only | Next.js 16, TypeScript | 3002 |
| `zellaweb` | Public website, pricing, sign-up, developer docs | Anyone | Next.js 16, TypeScript | 3000 |

The backend is the only project that holds money logic. The three frontends
display data and send commands. They never compute balances, fees or
transaction states on their own.

## Where they live

Repositories are checked out as siblings. From any project root:

```
../zellapay        backend (Go)
../zellamerchant   merchant dashboard
../zellapayadmin   admin console
../zellaweb        public website
```

Locally these are folders on the Desktop of the development machine
(`~/Desktop/<project>`). Remote URLs are not recorded here until the GitHub
organisation exists; add them to the table in `docs/ecosystem/remotes.md`
when it does. Before relying on a sibling path, check that it exists with
`ls ..`.

## How they connect

```
 Browser ──▶ zellaweb          static-first, no authentication
 Browser ──▶ zellamerchant ──▶ zellapay  /dashboard/*  (merchant session)
 Browser ──▶ zellapayadmin ──▶ zellapay  /admin/*      (staff session)
 Merchant servers ───────────▶ zellapay  /v1/*         (API key)
 Operators ──────────────────▶ zellapay  callbacks     (signed)
```

- **Backend for frontend.** The dashboards are Next.js servers. The browser
  talks only to its own Next.js server. That server calls the ZellaPay API
  with a server-side session. Session tokens are never exposed to browser
  JavaScript, and the API needs no browser CORS for them.
- **Three route groups, three authentication systems.** `/v1` uses API keys,
  `/dashboard` uses merchant user sessions, `/admin` uses staff sessions.
  They never share credentials or cookies.
- **One contract.** `api/openapi.yaml` in this repository describes `/v1`.
  The `/dashboard` and `/admin` contracts are planned and will live beside it.

## Status

| Area | State |
|---|---|
| `/v1/collections` endpoints | Built, tested end to end, in the OpenAPI contract |
| Webhook endpoints, signed delivery with retries and replay | Built and tested end to end |
| Payouts (hold, send, capture or release) and settlement | Built and tested end to end |
| Other `/v1` endpoints (refunds, balance, operators) | In the contract, handlers not built |
| Ledger, merchants, API keys, idempotency, simulator | Built and tested |
| Collections (state machine, ledger posting, operator confirmation, recovery sweeper) | Built and tested |
| `/dashboard` and `/admin` backend routes | Not designed yet |
| Dashboard and admin authentication (people, MFA, roles) | Not designed yet |
| `zellamerchant`, `zellapayadmin`, `zellaweb` | Scaffolded with docs, no features |

Do not build frontend screens that depend on a route group whose contract does
not exist. Write the contract first, in the backend.

## Rules that apply everywhere

- No file over 300 lines. No Go or TypeScript line over 100 characters. No
  comment line over 80 characters. Enforced by each project's check script.
- Money is an integer in the smallest currency unit. Never a float.
- Secrets and personal data never appear in logs, URLs, analytics or client
  bundles.
- Contract first: change the OpenAPI file, then the backend, then the clients.
- Each project has its own `CLAUDE.md`. Read it before changing anything.

## Changing something that spans projects

1. Update the contract in `zellapay/api` and its contract tests.
2. Implement and test the backend change. Release a backend version.
3. In each affected frontend, regenerate the API client from that version and
   implement the UI.
4. Note the change in each project's changelog or pull request.

See `integration-contract.md` for the details of steps 1 and 3, and
`local-development.md` for running everything together.
