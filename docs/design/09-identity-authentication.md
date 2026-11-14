# Identity and access: authentication

Status: Proposed.

## Passwords

- Hashed with **argon2id** and a per-user salt. Starting parameters: 64 MiB of
  memory, 3 passes, 2 lanes. They are tuned by benchmark on production
  hardware to take roughly a quarter of a second, and reviewed yearly.
- The parameters are stored with each hash. A user whose hash uses older
  parameters is rehashed at their next login.
- A secret **pepper** held in the key management service is mixed in with
  HMAC before hashing, so a stolen database alone is not enough to crack
  passwords. It can be rotated by keeping a version beside each hash.
- Rules: at least 12 characters, at most 128, no composition rules, Unicode
  normalised (NFKC), never truncated. Common and breached passwords are
  refused using the Have I Been Pwned range API, which only receives the
  first five characters of a hash. Passwords are never logged.

## Signing in

1. The user submits email and password.
2. The server checks the password in roughly constant time, whether or not the
   account exists, and answers with one generic error on any failure.
3. If MFA is enrolled, a second step verifies a code or a passkey.
4. A session is created. The user is told by email about sign-ins from a new
   device.

### Throttling and lockout

| Control | Rule |
|---|---|
| Per account | After 5 failures, delay grows 1, 2, 4, 8, 15 minutes. The account is not disabled, so an attacker cannot lock a real user out for long. |
| Per source address | A sliding window limit on attempts and on distinct accounts tried |
| Global | An alert when failures spike, which usually means credential stuffing |
| Bot check | A challenge after repeated failures. The provider is an open question. |

Counters live in Redis. Responses never reveal whether an email is registered.
The same applies to password reset and invitation endpoints.

## Multi-factor authentication

**Merchants:** an authenticator app using TOTP (RFC 6238: six digits, 30
seconds, one step of drift allowed). Enrolment is mandatory before a merchant
can do anything beyond editing their profile.

- The secret is stored encrypted, bound to the user. A code is accepted once:
  the last used time step is remembered so a captured code cannot be replayed.
- Ten **recovery codes** are shown once at enrolment, stored hashed, and each
  works once. Using one prompts the user to re-enrol.
- Push-style approval is not used, because it invites MFA fatigue attacks.

**Staff:** **WebAuthn** (hardware keys or device passkeys) is required. Each
staff user registers at least two credentials. WebAuthn resists phishing,
which matters most for the accounts that can approve merchants and release
funds. There is no TOTP fallback for staff.

**Step-up:** certain actions require MFA within the last ten minutes, even in
a fresh session: creating or rotating API keys, changing payout settings,
inviting users or changing roles, sending payouts above a threshold from the
dashboard, and changing the account's own credentials.

### Lost second factor

A merchant uses a recovery code. With none left, staff reset MFA only after
verifying identity out of band, with two staff approving, and the reset is
audited and emailed to the account owner. There is no self-service bypass.

## Sessions

- The session token is 256 random bits. Only its SHA-256 hash is stored.
- Merchant sessions expire after 30 minutes idle or 12 hours in total. Staff
  sessions expire after 15 minutes idle or 8 hours.
- A new token is issued at login and whenever privileges change. Changing a
  password or resetting MFA revokes every other session.
- Users can list their sessions and end any of them. Staff can end a user's
  sessions from the admin console.
- The frontend cookie is `__Host-` prefixed, `Secure`, `HttpOnly`, and
  `SameSite=Lax` (`Strict` for the admin console). State-changing requests
  also check `Origin`.
- Sessions are not pinned to an IP address, because mobile networks change
  them. A change is logged, and a very different location triggers re-auth.

## Invitations, verification and reset

| Token | Lifetime | Notes |
|---|---|---|
| Invitation | 7 days | Single use; binds email, role and merchant |
| Email verification | 24 hours | Single use |
| Password reset | 30 minutes | Single use; revokes all sessions; does not bypass MFA |

All are 256-bit random values, stored hashed, compared in constant time, and
invalidated when used or replaced. Links never contain personal data.

## Notifications

One interface sends everything the system must tell a person:

```go
type Notifier interface {
    Send(ctx context.Context, m Message) error
}
```

A `Message` names a template, a channel (email or SMS), a recipient and the
template's data. Templates are versioned and exist in French and English.

Sending goes through a **notifications outbox**: the message is stored in the
same transaction as the event that caused it, and a worker delivers it with
retries. Signing up never fails because a mail server is down.

| Environment | Email | SMS |
|---|---|---|
| Development | SMTP to **MailDev**; inbox at `http://localhost:1081` | **Written to the log**, including the code, so developers can continue |
| Sandbox | A real provider, restricted to approved test addresses | Written to the log |
| Production | A real provider | A real gateway |

Production refuses to start unless real providers are configured, in the same
way it refuses to start with the simulator enabled. In production, message
bodies and codes are never logged.

**SMS is not used as an authentication factor.** SIM-swap attacks make it weak
for that. It is for alerts: new sign-in, password change, payout sent,
suspicious activity, and later phone verification.

### Local setup

MailDev runs in Docker Compose: SMTP on port 1026 and the inbox on 1081.
Both are bound to `127.0.0.1` and use non-default ports to avoid clashing with
other projects on the same machine.
