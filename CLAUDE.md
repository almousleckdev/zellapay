# ZellaPay engineering standards

ZellaPay moves real money for real merchants. A defect here loses funds,
leaks personal data or breaks a regulated flow. This is not a tutorial or a
prototype. Every change is written, reviewed and tested as production code.

Read `docs/DESIGN.md` and `docs/adr/` before changing architecture.
Scoped rules live in `CLAUDE.md` files inside `internal/ledger`,
`internal/operators` and `db`. They add to this file and never relax it.

## Hard limits (enforced by `make check-limits` and CI)

- A file never exceeds 300 lines. Aim for under 200. Split by responsibility.
  This applies to code, tests, SQL, YAML and Markdown.
- A Go line never exceeds 100 characters. A comment line never exceeds 80.
- A function stays under 40 lines and does one thing.
- Cyclomatic complexity per function stays under 10.
- A function takes at most 4 parameters. Group more into a typed struct.
- Generated files are exempt and must start with `Code generated`.
- Never raise a limit or add an exemption to pass a check. Refactor.

## Principles

- **Single responsibility.** One reason to change per package, type and
  function. If you need "and" to describe it, split it.
- **DRY, with judgment.** Remove real duplication of knowledge. Do not merge
  code that only looks alike. Three similar lines beat a wrong abstraction.
- **Clean architecture.** Dependencies point inward: transport, then
  application, then domain. The domain imports no framework, no database
  driver and no HTTP package.
- **Small interfaces defined by the consumer.** Return concrete types.
- **Explicit over clever.** No reflection tricks, global state, `init()`
  side effects or hidden control flow.
- **Make illegal states unrepresentable.** Use types for money, IDs, states
  and phone numbers instead of raw `string` and `int64`.
- **Fail loudly and early.** Validate at the boundary, then trust the types.

## Architecture rules

- Modular monolith. Three binaries: `api`, `worker`, `operator-gateway`.
- A module owns its tables. No module reads or writes another module's
  tables. Cross-module calls go through the other module's exported interface.
- `internal/ledger` imports no other domain module.
- No import cycles. No `utils`, `common`, `helpers` or `misc` packages.
  Name packages after what they provide.
- `cmd/*` only wires dependencies and starts processes. No business logic.
- Business rules live in the domain layer and are testable without I/O.
- Put side effects (database, network, clock, randomness) behind interfaces
  and inject them. Time comes from a `Clock`, never `time.Now()` in domain code.

## Money rules (non-negotiable)

- Amounts are `int64` in the smallest unit with a currency. Never a float.
- Every journal balances per currency. Entries are append-only.
  Corrections are reversing entries. Never update or delete an entry.
- Never compute or store a balance by overwriting it. Derive it from entries.
- Every money-moving write is idempotent and runs in one database transaction.
- Lock accounts in a fixed order to prevent deadlocks. Retry on serialization
  failure with a bound.
- A timeout from an operator is `unknown`, never `failed`. Never release or
  re-send funds on an unknown outcome. Resolve it by querying or reconciling.
- Terminal transaction states never change. A contradicting late signal opens
  a reconciliation case. It does not rewrite history.
- Rounding rules are explicit, documented and tested. No implicit rounding.

## Security

Treat every input, operator callback and merchant request as hostile.

**Cryptography**
- Use only the Go standard library and `golang.org/x/crypto`. Never write
  custom cryptography. Never use `math/rand` for anything security related.
  Use `crypto/rand`.
- Encrypt sensitive data at rest with AES-256-GCM or XChaCha20-Poly1305,
  using envelope encryption. Data keys are wrapped by a KMS-managed key.
  Store a key ID with every ciphertext so keys can rotate.
- Hash passwords with argon2id and per-user salts. Tune cost by benchmark.
- API keys are 256-bit random values. Store only a hash and a short prefix.
  Show the key once. Compare with `crypto/subtle.ConstantTimeCompare`.
- Sign webhooks with HMAC-SHA256 over timestamp and body. Verify with
  `hmac.Equal`. Reject stale timestamps to stop replay.
- TLS 1.3 minimum on public endpoints. mTLS between internal services.
- Lookup of encrypted fields (such as a phone number) uses a keyed blind
  index (HMAC), never the plaintext.

**Data and secrets**
- Never log secrets, keys, tokens, full phone numbers, personal data or
  raw request bodies. Mask by default: `+241 07 ** ** 42`.
- No secrets in code, tests, fixtures, images, CI logs or environment
  files committed to git. Load them from the secret manager.
- Personal data is minimised, classified and has a retention rule.
- Errors returned to clients carry a stable code and request ID, never
  internals, SQL, stack traces or secrets.

**Application**
- Parameterised queries only, through `sqlc`. No string-built SQL.
- Validate and bound every input: size, length, range, format, enum.
  Set `http.MaxBytesReader`, server timeouts and per-client rate limits.
- Authorise every request against the merchant that owns the resource.
  Never trust an ID from the client without an ownership check.
- Outbound calls (webhooks, operators) have deadlines, size limits and
  SSRF protection: block private and loopback ranges, resolve then pin.
- Database roles are least privilege. The `api` role cannot alter ledger
  history. Migrations run under a separate role.
- Run `govulncheck`, `staticcheck` and `gosec` in CI. Pin dependencies and
  keep the dependency list short. Justify every new dependency in the PR.

## Clean code conventions

- Names say what and why. No abbreviations beyond the common (`ctx`, `id`).
- Comments explain why, not what. Keep them short and accurate. Delete
  commented-out code. No TODO without a linked issue.
- Every exported identifier has a doc comment starting with its name.
- Errors: wrap with context using `%w`, define sentinel or typed errors for
  conditions callers must handle, never discard an error, never `panic` in
  request paths.
- `context.Context` is the first parameter of anything that does I/O, and
  every I/O call has a deadline.
- No goroutine without an owner, a stop signal and error handling.
- Prefer early returns and flat control flow over nesting.
- Table-driven tests. Test names state behaviour.
- Logging uses `log/slog` with structured fields and a request ID.

## Testing

- No change merges without tests. Domain rules need unit and property
  tests. Persistence needs integration tests on real Postgres through
  testcontainers. Concurrency code runs with `-race`.
- Ledger invariants are tested continuously: journals balance, no negative
  available balance, idempotent replays do not double post.
- Parsers (callbacks, API bodies) have fuzz tests.
- Tests are deterministic: injected clock, seeded or injected randomness,
  no sleeping for synchronisation.
- Target 90 percent coverage in domain packages. Coverage is a floor, not
  a goal. Test behaviour and failure paths.

## Database

- Forward-only migrations, one concern each, reviewed like code.
  Never edit an applied migration.
- Every migration is safe to run during a rolling deploy: add, backfill,
  then switch, then drop in a later release.
- Constraints belong in the database: `NOT NULL`, `CHECK`, foreign keys,
  unique indexes. Do not rely only on application checks.
- Index for real queries. Review plans with `EXPLAIN` on realistic data.

## API and contracts

- `api/openapi.yaml` is the contract. Change it first, then the code.
- Never break a published version. Add fields, never remove or repurpose.
- Stable machine-readable error codes. One error envelope everywhere.
- Every POST requires an `Idempotency-Key`.

## Workflow for Claude

1. Read the relevant design doc, ADR and scoped `CLAUDE.md` first.
2. State the plan briefly. For anything touching money flow, schema or
   security, confirm the approach with the user before writing code.
3. Write the test first for domain logic.
4. Keep each change small and focused. One concern per commit.
5. Before declaring done, run: `make check-limits vet race lint`.
   Report failures plainly. Never claim success without running them.
6. Never commit, push or deploy without being asked.
7. Never disable a check, skip a test, or weaken a rule to get green.
8. If a rule here conflicts with the request, say so and ask.

## Definition of done

- Behaviour matches the design and the OpenAPI contract.
- Tests cover success, failure and edge cases and pass with `-race`.
- No file or line limit is violated. No lint or vulnerability findings.
- No secret or personal data in code, logs or fixtures.
- Docs, ADRs and runbooks are updated when behaviour or operations change.
- A reviewer could operate and debug this at 3 a.m. from the logs alone.
