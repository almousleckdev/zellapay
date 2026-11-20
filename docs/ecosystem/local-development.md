# Local development across projects

## Ports

| Project | Port |
|---|---|
| `zellaweb` | 3000 |
| `zellamerchant` | 3001 |
| `zellapayadmin` | 3002 |
| `zellapay` api | 8080 |
| PostgreSQL (compose) | 5433 |
| Redis (compose) | 6380 |
| MailDev SMTP (compose) | 1026 |
| MailDev inbox (compose) | 1081 |

The database and cache use non-default ports on purpose so they do not clash
with other projects on the same machine. Both bind to `127.0.0.1` only.

## Email in development

Every email the application sends in development lands in MailDev, and nothing
is delivered to a real address. Start it with `make up`, then open the inbox at
`http://localhost:1081`. SMS messages are written to the application log
instead. To see it work, run `go run ./cmd/zellactl notify test you@example.test`. See `docs/design/09-identity-authentication.md`.

## Start the backend

```bash
cd ../zellapay
make up        # env file, PostgreSQL, Redis, MailDev, migrations, dev staff account
make run-api   # http://localhost:8080
```

The first `make up` creates `.env.local` and a staff account for the admin
console (email and password are `ZP_DEV_STAFF_*` in `.env.local`). Later runs
change nothing. `make reset` throws the local database away and starts over.

Interactive API documentation is at `http://localhost:8080/docs/` outside
production.

## Start a frontend

```bash
cd ../zellamerchant
pnpm install
cp .env.example .env.local   # then fill in values
pnpm dev                     # http://localhost:3001
```

Requires Node 20 or newer and pnpm 10.

## Checks before review

| Project | Command |
|---|---|
| `zellapay` | `make check` |
| frontends | `pnpm check` |

Both run the file and line limit script, linting, type checks and tests.

## Environment variables

Each project documents its variables in `.env.example`. Real values live only
in `.env.local` (git-ignored) or the secret manager. Never commit them and
never prefix a secret with `NEXT_PUBLIC_`, which publishes it to browsers.

## Working across projects

Open the project you are changing as the working directory. Read sibling
repositories for context only; make changes in them from their own working
directory so their `CLAUDE.md` rules apply.

## Test phone numbers

The operator simulator selects an outcome from the last two digits of the
phone number. See `docs/simulator.md` in the backend repository.

## Client addresses and trusted proxies

The API believes `X-Forwarded-For` only from the networks in
`ZP_TRUSTED_PROXIES` (CIDR ranges). Locally it trusts loopback, so a dashboard
on the same machine can report its users' addresses. Production trusts nobody
until you list the load balancer and dashboard ranges; `0.0.0.0/0` is
refused. The dashboard sends the address from the header named by its
`CLIENT_IP_HEADER`. The API uses the result for per-address throttling, API
key allowlists and the session list.
