# Identity and access: overview

Status: Proposed. Review before implementation.

This design covers how people sign in to ZellaPay: merchants and their staff
through the merchant dashboard, and ZellaPay's own staff through the admin
console. It does not change the merchant API, which keeps authenticating
servers with API keys (`/v1`).

## Goals

- A merchant can be invited, set up an account, enrol MFA, apply for live
  access and, once approved, run their business from the dashboard.
- ZellaPay staff can review and approve merchants, resolve held transactions
  and administer the platform, with every action attributable and audited.
- A stolen password, a stolen session or a careless insider cannot move money
  on their own.

## Non-goals (for now)

- Single sign-on, OAuth for third-party apps, and social login.
- Customer (end user) accounts. Customers only approve payments on their phone.
- SMS as an authentication factor (see `09-identity-authentication.md`).

## Principals

| Principal | Authenticates with | Reaches | Lives in |
|---|---|---|---|
| Merchant server | API key | `/v1` | `merchant_api_keys` (built) |
| Merchant user | Password and MFA | `/dashboard` | `merchant_users` |
| Staff user | Password and WebAuthn | `/admin` | `staff_users` |

The three never share credentials, tables, cookies or sessions. A merchant
user can never become staff, and a staff user is never a merchant user.

## Architecture

```
Browser ──▶ zellamerchant (Next.js) ──▶ backend /dashboard
Browser ──▶ zellapayadmin (Next.js) ──▶ backend /admin
```

- The browser talks only to its own Next.js server (backend for frontend).
- On login the backend issues an opaque session token. The Next.js server
  stores it in an encrypted, `HttpOnly` cookie and sends it to the backend as
  a bearer credential. Browser JavaScript never sees it.
- The backend is the only place that decides who may do what. The frontends
  hide buttons for convenience, never for security.
- Every `/dashboard` request derives the merchant from the session. A merchant
  ID sent by the browser is never trusted.

## Environments

Sandbox and production are separate deployments with separate databases, so a
person has a separate account in each. Sandbox holds no real money, so it can
open to self-service sign-up later, behind bot protection. Production is by
invitation only: it is created for a merchant when staff approve their
application.

## Merchant lifecycle

```
invited ──▶ account set up ──▶ MFA enrolled ──▶ application submitted
                                                       │
                       needs_info ◀── in_review ◀──────┘
                           │              │
                           └──▶ resubmit  ├──▶ rejected
                                          └──▶ approved ──▶ live
```

| Stage | What the merchant can do |
|---|---|
| Invited | Nothing until they accept and set a password |
| Account set up, MFA enrolled | Use the sandbox, read documentation, edit the application |
| Application submitted | Same, while staff review |
| Approved | Live API keys, payouts and the full dashboard in production |
| Suspended | Read-only; no new money movement; staff can restore |

Approval creates the production merchant and sends its owner an invitation.
Fees and limits are set by staff at approval and recorded in the audit log.

## Data model

Tables, with the important columns. Secrets and tokens are stored only as
hashes or encrypted values.

| Table | Purpose |
|---|---|
| `merchant_users` | Email, role, status, password hash, MFA secret (encrypted), failed-login counters |
| `staff_users` | Same, with WebAuthn credentials instead of TOTP |
| `webauthn_credentials` | Staff passkeys and hardware keys, with sign counters |
| `invitations` | Single-use invitation tokens (hash), role, expiry, who invited |
| `email_tokens` | Verification and password-reset tokens (hash), purpose, expiry, used time |
| `sessions` | Hash of the session token, user, issue and last-seen times, MFA time, revoked time |
| `recovery_codes` | Hashed single-use MFA recovery codes |
| `merchant_applications` | Business details, status, reviewer decisions with reasons |
| `application_documents` | Metadata for documents kept encrypted in object storage |
| `approvals` | Pending and decided four-eyes approvals |
| `audit_log` | Append-only, hash-chained record of every sensitive action |
| `notifications` | Outbox of emails and SMS to send, with status and retries |

Existing tables stay as they are. `merchants.status` gains the `suspended`
behaviour described above.

## Where this lives in the code

| Package | Responsibility |
|---|---|
| `internal/identity` | Users, passwords, MFA, sessions, invitations, recovery |
| `internal/access` | Roles, permissions and the policy check used by every handler |
| `internal/onboarding` | Applications, documents, review and approval |
| `internal/audit` | The audit log and its integrity check |
| `internal/notify` | Email and SMS sending behind one interface |
| `internal/identity/httpapi`, `internal/onboarding/httpapi` | `/dashboard` and `/admin` routes |

Modules follow the existing rules: each owns its tables and talks to others
through interfaces.
