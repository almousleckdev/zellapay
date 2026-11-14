# Identity and access: roles, approval and audit

Status: Proposed.

## Roles

Permissions are named capabilities such as `payouts:create`. A role is a fixed
set of permissions. The mapping lives in code, is covered by a table-driven
test, and is enforced by the backend on every request.

### Merchant roles

| Permission | Owner | Admin | Developer | Finance | Support |
|---|:-:|:-:|:-:|:-:|:-:|
| View transactions, balances, statements | ✓ | ✓ | ✓ | ✓ | ✓ |
| Create payouts | ✓ | ✓ | | ✓ | |
| Manage API keys and webhooks | ✓ | ✓ | ✓ | | |
| Invite users, change roles | ✓ | ✓ | | | |
| Edit business profile, settlement details | ✓ | | | | |
| Transfer ownership, close the account | ✓ | | | | |

Every merchant has exactly one owner. Support is read-only. Payout settings
and ownership changes always require step-up MFA.

### Staff roles

| Role | Purpose |
|---|---|
| `support` | Look up merchants and transactions, with personal data masked |
| `risk_analyst` | Review flagged payouts, resolve held transactions, suspend a merchant |
| `compliance` | Review and decide merchant applications |
| `finance` | Reconciliation, statements, fee reports |
| `admin` | Staff management, fee and limit settings |
| `super_admin` | A small group for emergencies; every use alerts the others |

A person can hold several roles. Roles are granted by an `admin`, with a
second approval for `admin` and `super_admin`.

## Four-eyes approvals

A second staff member must approve these actions. The approver cannot be the
requester, and both are recorded:

- Approving a merchant for live access.
- Releasing a payout above the review threshold.
- Changing a merchant's fees or limits.
- Granting or removing a staff role.
- Resetting a user's MFA.
- Resolving a transaction held for an amount mismatch.

Approvals expire after 24 hours and cannot be reused.

## Merchant onboarding

A merchant completes an application:

- Business: legal name, registration and tax numbers, address, sector.
- People: the owner and beneficial owners, with identity documents.
- Settlement: where payouts go, and expected volumes.
- Documents: company registration and identity files.

The exact fields and retention periods depend on Gabon and CEMAC regulation
and are an **open question for your legal and compliance advisers**. This
design deliberately does not guess them.

Handling rules that do not depend on the answer:

- Documents are stored in a private object-storage bucket with server-side
  encryption, never served directly. They are scanned, limited in size and
  type, and downloaded only through short-lived signed links.
- Staff see documents only when working an application, and each view is
  audited.
- Review: `compliance` records a recommendation with a reason, then a second
  staff member approves. Outcomes are approved, rejected with a reason, or
  needs-more-information.
- Screening against sanctions and politically exposed person lists happens at
  approval. The provider is an open question.
- Approval records the fees and limits granted, creates the production
  merchant and sends the owner an invitation.

## Admin safeguards

- Reachable only from a private network or an allowlisted address, enforced at
  the edge, in addition to application login.
- Every state-changing action asks for a typed reason, which is stored.
- No impersonation: staff never act as a merchant. Support uses read-only
  views.
- Personal data is masked by default. Unmasking is one action with a reason,
  and is audited.
- Nothing is deleted. Corrections are new records.

## Audit log

Every sign-in outcome, MFA change, session end, key issue or revoke, role
change, application decision, approval, fee change, resolution and unmask is
recorded.

| Field | Content |
|---|---|
| Actor | Kind (merchant user, staff, API key, system) and ID |
| Action | A stable name such as `application.approved` |
| Target | The kind and ID of what was affected |
| Reason | Required for staff actions |
| Change | What changed, as identifiers and non-personal values |
| Context | Time, source address, user agent, request ID |

The log is append-only, enforced by the database. Each row stores a hash of
the previous row, so any edit or deletion breaks the chain. A scheduled job
verifies the chain and alerts on a break, and a copy is exported to
write-once storage. Personal data is never written into the log; it refers to
records by ID.

## Threats and how this design answers them

| Threat | Response |
|---|---|
| Credential stuffing | Throttling, breached-password check, bot challenge, alerts |
| Phishing of staff | WebAuthn, which cannot be replayed on a fake site |
| Session theft | Opaque tokens, `HttpOnly` encrypted cookie, short sessions, step-up |
| Cross-site request forgery | `SameSite`, `Origin` checks, server-side actions |
| Account enumeration | Identical responses and timing for unknown accounts |
| Stolen API key moving money | Risk controls on payouts (a separate piece of work) |
| Insider misuse | Roles, four-eyes, mandatory reasons, masked data, hash-chained audit |
| Privilege escalation | Permission checks in the backend, tested as a table |
| Insecure direct object reference | The merchant comes from the session; every query is scoped |
| Lost second factor | Recovery codes, then two-person staff-assisted reset |
