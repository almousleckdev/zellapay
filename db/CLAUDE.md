# Database rules

Apply the root `CLAUDE.md` plus the rules below.

## Migrations

- Forward-only, numbered, one concern per file, under 300 lines.
- Never edit a migration that has been applied anywhere.
- Rolling-deploy safe: add, backfill, switch, then drop in a later release.
- Long operations use `CREATE INDEX CONCURRENTLY` and batched backfills.
- Every table has a primary key, `created_at`, and `NOT NULL` by default.
- Money columns are `bigint` with a `CHECK` where a sign rule applies,
  plus a `char(3)` currency column.
- Ledger tables: no `UPDATE` or `DELETE` privilege for any application
  role, and a trigger that rejects both.

## Roles

- `migrator`: schema changes only, used by the deploy pipeline.
- `api`, `worker`, `operator_gateway`: least privilege per schema.
- No application role is a superuser or owns tables.

## Queries

- Written in `queries/` and compiled by `sqlc`. No SQL strings in Go.
- Select explicit columns. No `SELECT *`.
- Keyset pagination for lists. No offset on large tables.
- Every query that must be consistent runs inside an explicit transaction
  with the weakest sufficient isolation level, and retries on serialization
  failure.

## Sensitive data

- Personal data is encrypted at the application layer with envelope
  encryption. The column stores ciphertext and a key ID.
- Searchable fields use a keyed blind index column. Never index plaintext.
- Retention and deletion are scheduled jobs with audit records.
