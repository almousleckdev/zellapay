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

The database and cache use non-default ports on purpose so they do not clash
with other projects on the same machine. Both bind to `127.0.0.1` only.

## Start the backend

```bash
cd ../zellapay
make up        # PostgreSQL and Redis
make run-api   # http://localhost:8080
```

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
