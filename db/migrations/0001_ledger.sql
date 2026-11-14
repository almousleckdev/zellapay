-- +goose Up
-- Ledger: accounts, journals and append-only entries.

CREATE TABLE ledger_accounts (
    id         text PRIMARY KEY CHECK (id <> ''),
    code       text NOT NULL UNIQUE CHECK (code <> ''),
    type       text NOT NULL
               CHECK (type IN ('asset', 'liability', 'revenue', 'expense')),
    currency   char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    allow_overdraft boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, currency)
);

CREATE TABLE ledger_journals (
    id              text PRIMARY KEY CHECK (id <> ''),
    idempotency_key text NOT NULL UNIQUE CHECK (idempotency_key <> ''),
    kind            text NOT NULL CHECK (kind <> ''),
    reference       text NOT NULL CHECK (reference <> ''),
    fingerprint     bytea NOT NULL CHECK (length(fingerprint) = 32),
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE ledger_entries (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    journal_id text NOT NULL REFERENCES ledger_journals (id),
    account_id text NOT NULL,
    side       text NOT NULL CHECK (side IN ('debit', 'credit')),
    amount     bigint NOT NULL CHECK (amount > 0),
    currency   char(3) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (account_id, currency)
        REFERENCES ledger_accounts (id, currency)
);

CREATE INDEX ledger_entries_account_idx
    ON ledger_entries (account_id, id);
CREATE INDEX ledger_entries_journal_idx
    ON ledger_entries (journal_id);

-- +goose StatementBegin
CREATE FUNCTION ledger_reject_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'ledger tables are append-only (% on %)',
        TG_OP, TG_TABLE_NAME USING ERRCODE = 'integrity_constraint_violation';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER ledger_entries_immutable
    BEFORE UPDATE OR DELETE ON ledger_entries
    FOR EACH ROW EXECUTE FUNCTION ledger_reject_change();
CREATE TRIGGER ledger_entries_no_truncate
    BEFORE TRUNCATE ON ledger_entries
    FOR EACH STATEMENT EXECUTE FUNCTION ledger_reject_change();
CREATE TRIGGER ledger_journals_immutable
    BEFORE UPDATE OR DELETE ON ledger_journals
    FOR EACH ROW EXECUTE FUNCTION ledger_reject_change();
CREATE TRIGGER ledger_journals_no_truncate
    BEFORE TRUNCATE ON ledger_journals
    FOR EACH STATEMENT EXECUTE FUNCTION ledger_reject_change();

-- +goose StatementBegin
CREATE FUNCTION ledger_assert_balanced() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    unbalanced record;
BEGIN
    SELECT currency,
           sum(CASE side WHEN 'debit' THEN amount ELSE -amount END) AS net
      INTO unbalanced
      FROM ledger_entries
     WHERE journal_id = NEW.journal_id
     GROUP BY currency
    HAVING sum(CASE side WHEN 'debit' THEN amount ELSE -amount END) <> 0
     LIMIT 1;
    IF FOUND THEN
        RAISE EXCEPTION 'journal % is unbalanced in %',
            NEW.journal_id, unbalanced.currency
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER ledger_entries_balanced
    AFTER INSERT ON ledger_entries
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION ledger_assert_balanced();
