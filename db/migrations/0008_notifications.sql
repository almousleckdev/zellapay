-- +goose Up
-- Outbox of emails and SMS. The recipient and any link or code are stored
-- sealed, and erased once the message is sent or abandoned.

CREATE TABLE notifications (
    id               text PRIMARY KEY CHECK (id <> ''),
    channel          text NOT NULL CHECK (channel IN ('email', 'sms')),
    template         text NOT NULL CHECK (template <> ''),
    locale           text NOT NULL CHECK (locale IN ('fr', 'en')),
    recipient_masked text NOT NULL CHECK (recipient_masked <> ''),
    payload_enc      text NOT NULL,
    dedupe_key       text UNIQUE,
    status           text NOT NULL CHECK (status IN ('pending', 'sent', 'dead')),
    attempts         integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at  timestamptz NOT NULL,
    retry_until      timestamptz NOT NULL,
    last_error       text NOT NULL DEFAULT '',
    created_at       timestamptz NOT NULL,
    sent_at          timestamptz,
    CHECK (status <> 'pending' OR payload_enc <> '')
);

CREATE INDEX notifications_due_idx
    ON notifications (next_attempt_at) WHERE status = 'pending';

-- +goose StatementBegin
CREATE FUNCTION notifications_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status <> 'pending' AND
       (NEW.status IS DISTINCT FROM OLD.status OR NEW.payload_enc <> '') THEN
        RAISE EXCEPTION 'notification % is final', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER notifications_final_guard
    BEFORE UPDATE ON notifications
    FOR EACH ROW EXECUTE FUNCTION notifications_guard();
