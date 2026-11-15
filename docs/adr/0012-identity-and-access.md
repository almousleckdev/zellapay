# ADR 0012: Identity and access for dashboards

Status: Proposed
Date: 2026-11-14

## Context
Merchants need to register, sign in and run their business, and ZellaPay staff
need to approve merchants and resolve held money. Both can move or stop real
money, so the accounts that reach them are the most attractive target.

## Decision
The full design is in `docs/design/08` to `11`. The decisions that shape
everything else:

1. **Three separate principals.** API keys, merchant users and staff users
   never share credentials, tables, cookies or sessions.
2. **Backend decides.** Permissions are checked in the backend on every
   request. Frontends reflect them and are never trusted.
3. **Opaque server-side sessions.** A 256-bit token, stored hashed, carried by
   the frontend server in an encrypted `HttpOnly` cookie. Short idle and
   absolute lifetimes, revocable at any time.
4. **Strong factors where it matters.** Merchants use TOTP. Staff must use
   WebAuthn with two credentials, which resists phishing. SMS is never an
   authentication factor.
5. **Passwords.** argon2id with a secret pepper, a 12 character minimum, and a
   breached-password check.
6. **Four-eyes for high-impact actions**, with mandatory reasons.
7. **Append-only, hash-chained audit log** of every sensitive action.
8. **One notification layer.** Email and SMS go through a single interface and
   a transactional outbox. Development uses MailDev for email and the log for
   SMS. Production refuses to start without real providers.
9. **Invitation-only production.** Sandbox and production are separate
   deployments with separate accounts. Sandbox may open to self-service later.

## Consequences
- Nothing in the frontends can be built against `/dashboard` or `/admin` until
  their OpenAPI contracts exist.
- KYC fields, screening and retention depend on regulation and are left open
  for legal advisers rather than guessed.
- Risk controls on payouts are a separate piece of work, but must be finished
  before any live launch.
- Production needs an email provider, an SMS gateway and a bot-protection
  provider chosen before launch.

## Addendum: TOTP uses HMAC-SHA1

Authenticator apps compute codes with HMAC-SHA1, and Google Authenticator
ignores any other algorithm. HMAC-SHA1 is not weakened by SHA-1 collisions.
`internal/platform/totp` therefore uses it, and `.golangci.yml` exempts that
one file from gosec rule G505. It is the only lint exemption in the project.
Moving to SHA-256 would lock out Google Authenticator users.
