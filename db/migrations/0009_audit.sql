-- +goose Up
-- Tamper-evident audit log. Entries are chained by hash and can only be added.
-- The database refuses any insert that does not extend the current head, so
-- the chain cannot fork even if the application has a bug.

CREATE TABLE audit_log (
    seq         bigint PRIMARY KEY CHECK (seq > 0),
    at          timestamptz NOT NULL,
    actor_kind  text NOT NULL
                CHECK (actor_kind IN ('merchant_user', 'staff', 'api_key', 'system')),
    actor_id    text NOT NULL CHECK (actor_id <> ''),
    action      text NOT NULL CHECK (action <> ''),
    target_kind text NOT NULL CHECK (target_kind <> ''),
    target_id   text NOT NULL CHECK (target_id <> ''),
    reason      text NOT NULL DEFAULT '',
    change      text NOT NULL DEFAULT '{}',
    ip          text NOT NULL DEFAULT '',
    user_agent  text NOT NULL DEFAULT '',
    request_id  text NOT NULL DEFAULT '',
    prev_hash   bytea NOT NULL CHECK (length(prev_hash) = 32),
    hash        bytea NOT NULL UNIQUE CHECK (length(hash) = 32)
);

CREATE INDEX audit_log_target_idx ON audit_log (target_kind, target_id, seq DESC);
CREATE INDEX audit_log_actor_idx ON audit_log (actor_kind, actor_id, seq DESC);
CREATE INDEX audit_log_action_idx ON audit_log (action, seq DESC);

-- The newest entry. Appending locks this single row, which orders all writers.
CREATE TABLE audit_head (
    id   smallint PRIMARY KEY CHECK (id = 1),
    seq  bigint NOT NULL,
    hash bytea NOT NULL CHECK (length(hash) = 32)
);

INSERT INTO audit_head (id, seq, hash) VALUES (1, 0, '\x0000000000000000000000000000000000000000000000000000000000000000');

-- How far verification has got, and where it found a break, if anywhere.
CREATE TABLE audit_verification (
    id            smallint PRIMARY KEY CHECK (id = 1),
    verified_seq  bigint NOT NULL,
    verified_hash bytea NOT NULL CHECK (length(verified_hash) = 32),
    verified_at   timestamptz NOT NULL,
    broken_seq    bigint NOT NULL DEFAULT 0,
    broken_reason text NOT NULL DEFAULT ''
);

INSERT INTO audit_verification (id, verified_seq, verified_hash, verified_at)
VALUES (1, 0, '\x0000000000000000000000000000000000000000000000000000000000000000', now());

-- +goose StatementBegin
CREATE FUNCTION audit_log_reject_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'the audit log is append-only (% attempted)', TG_OP
        USING ERRCODE = 'integrity_constraint_violation';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER audit_log_immutable
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_reject_change();
CREATE TRIGGER audit_log_no_truncate
    BEFORE TRUNCATE ON audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION audit_log_reject_change();

-- +goose StatementBegin
CREATE FUNCTION audit_log_extend_head() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    current_head audit_head%ROWTYPE;
BEGIN
    SELECT * INTO current_head FROM audit_head WHERE id = 1;
    IF NEW.seq <> current_head.seq + 1 OR NEW.prev_hash <> current_head.hash THEN
        RAISE EXCEPTION 'an audit entry must extend the current head'
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER audit_log_extends_head
    BEFORE INSERT ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_extend_head();

-- +goose StatementBegin
CREATE FUNCTION audit_head_follow_log() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.seq <> OLD.seq + 1 OR NOT EXISTS (
        SELECT 1 FROM audit_log WHERE seq = NEW.seq AND hash = NEW.hash
    ) THEN
        RAISE EXCEPTION 'the audit head must advance to an entry that exists'
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER audit_head_follows_log
    BEFORE UPDATE ON audit_head
    FOR EACH ROW EXECUTE FUNCTION audit_head_follow_log();
