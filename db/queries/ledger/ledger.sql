-- name: CreateAccount :exec
INSERT INTO ledger_accounts (id, code, type, currency, allow_overdraft)
VALUES (@id, @code, @type, @currency, @allow_overdraft);

-- name: LockAccounts :many
SELECT id, type, currency, allow_overdraft
  FROM ledger_accounts
 WHERE id = ANY(@ids::text[])
 ORDER BY id
   FOR UPDATE;

-- name: InsertJournal :one
INSERT INTO ledger_journals (id, idempotency_key, kind, reference, fingerprint)
VALUES (@id, @idempotency_key, @kind, @reference, @fingerprint)
ON CONFLICT (idempotency_key) DO NOTHING
RETURNING id;

-- name: GetJournalByKey :one
SELECT id, fingerprint
  FROM ledger_journals
 WHERE idempotency_key = @idempotency_key;

-- name: InsertEntries :exec
INSERT INTO ledger_entries (journal_id, account_id, side, amount, currency)
SELECT @journal_id::text,
       unnest(@accounts::text[]),
       unnest(@sides::text[]),
       unnest(@amounts::bigint[]),
       unnest(@currencies::text[]);

-- name: AccountTotals :many
SELECT account_id,
       COALESCE(sum(amount) FILTER (WHERE side = 'debit'), 0)::bigint
           AS debits,
       COALESCE(sum(amount) FILTER (WHERE side = 'credit'), 0)::bigint
           AS credits
  FROM ledger_entries
 WHERE account_id = ANY(@ids::text[])
 GROUP BY account_id;

-- name: AccountBalanceTotals :one
SELECT a.type,
       COALESCE(sum(e.amount) FILTER (WHERE e.side = 'debit'), 0)::bigint
           AS debits,
       COALESCE(sum(e.amount) FILTER (WHERE e.side = 'credit'), 0)::bigint
           AS credits
  FROM ledger_accounts a
  LEFT JOIN ledger_entries e ON e.account_id = a.id
 WHERE a.id = @id
 GROUP BY a.id, a.type;

-- name: EnsureAccount :exec
INSERT INTO ledger_accounts (id, code, type, currency, allow_overdraft)
VALUES (@id, @code, @type, @currency, @allow_overdraft)
ON CONFLICT DO NOTHING;

-- name: GetAccount :one
SELECT id, type, currency, allow_overdraft
  FROM ledger_accounts
 WHERE id = @id;
