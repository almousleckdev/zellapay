# Identity and access: API and plan

Status: Proposed.

The `/dashboard` and `/admin` route groups are separate OpenAPI documents
(`api/dashboard.openapi.yaml`, `api/admin.openapi.yaml`) with the same
contract tests as `/v1`. The frontends generate their clients from them.
Sketch of the first versions:

## `/dashboard` (merchant users)

| Area | Routes |
|---|---|
| Accounts | Accept invitation, verify email, request and confirm password reset |
| Sign in | Log in, verify MFA, log out, current session |
| Security | Change password, enrol and remove MFA, regenerate recovery codes, list and end sessions |
| Team | List users, invite, change role, remove |
| Business | View and edit profile, application: edit, upload document, submit |
| Keys and hooks | List, create, rotate, revoke API keys; webhook endpoints and event replay |
| Money | Transactions and balances, create payout (with step-up above a threshold), statements and exports |

## `/admin` (staff)

| Area | Routes |
|---|---|
| Sign in | WebAuthn begin and finish, log out, current session |
| Merchants | Search, view, suspend and restore, set fees and limits (four-eyes) |
| Applications | Queue, view with documents, recommend, approve (second approver), reject, request more information |
| Transactions | Look up a collection or payout, view its timeline, resolve held items |
| Staff | Invite, change roles (four-eyes), end sessions, reset MFA (four-eyes) |
| Audit | Search and export, verify the chain |
| Platform | Operator health, reconciliation breaks |

Rules that apply to every route:

- Authentication, then permission, then business logic.
- Idempotency keys on every `POST`, as in `/v1`.
- The same error envelope, with stable codes.
- No route accepts a merchant ID as authority.

## Delivery plan

Each step ships with tests and leaves the system working.

| Step | Delivers |
|---|---|
| 1 | `notify`: interface, MailDev email, logged SMS, outbox and worker. **Done.** |
| 2 | `audit`: append-only hash-chained log with an integrity check. **Done.** |
| 3 | `identity`: users, argon2id passwords, invitations, verification, reset. **Done.** Sessions revoke on reset in step 4. |
| 4 | Sessions, TOTP MFA with recovery codes, throttling and lockout. **Done.** |
| 5 | `access`: roles, permissions, step-up, and the policy test table. **Done.** |
| 6 | `/dashboard` routes and OpenAPI; `zellamerchant` sign-in and security pages |
| 7 | `onboarding`: application, documents, review and approval; `zellapayadmin` queue |
| 8 | Staff accounts with WebAuthn, four-eyes approvals, the rest of `/admin` |
| 9 | Risk controls on payouts and the held-transaction tools |

Risk controls (step 9) must be finished before any live launch.

## Decisions made

1. Production is invitation-only at first. Sandbox may open to self-service
   sign-up later, behind bot protection.
2. Sandbox and production are separate deployments with separate accounts.
3. Merchants use TOTP. Staff require WebAuthn with at least two credentials.
4. Email and SMS go through one `Notifier`. Development uses MailDev for email
   and the log for SMS.
5. Sessions are opaque, server-side, hashed at rest, and carried by the
   frontend in an encrypted `HttpOnly` cookie.
6. The audit log is append-only and hash-chained.

## Open questions

| Question | Who decides |
|---|---|
| Which KYC fields, documents and retention periods Gabon and CEMAC require | Legal and compliance advisers |
| Which sanctions and PEP screening provider | Compliance |
| Production email provider and SMS gateway, and their data-residency terms | You |
| Bot-protection provider for sign-in and sign-up | You |
| Whether the breached-password check may call an external service, or must use a local list | Security |
| WebAuthn relying-party domains per environment | Infrastructure |
| Whether one person may belong to several merchants | Product |
| The payout review threshold and the step-up threshold | Risk, with the business |
