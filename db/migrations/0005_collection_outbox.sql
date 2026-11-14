-- +goose Up
-- Transactional outbox: a row is written in the same transaction as every
-- final collection state, so a state change and its event cannot drift apart.

CREATE TABLE collection_outbox (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    collection_id text NOT NULL REFERENCES collections (id),
    event_type    text NOT NULL CHECK (event_type IN
        ('collection.succeeded', 'collection.failed', 'collection.expired')),
    created_at    timestamptz NOT NULL,
    published_at  timestamptz,
    UNIQUE (collection_id, event_type)
);

CREATE INDEX collection_outbox_unpublished_idx
    ON collection_outbox (id) WHERE published_at IS NULL;

-- +goose StatementBegin
CREATE FUNCTION collection_outbox_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'outbox rows are never deleted'
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    IF NEW.id IS DISTINCT FROM OLD.id
       OR NEW.collection_id IS DISTINCT FROM OLD.collection_id
       OR NEW.event_type IS DISTINCT FROM OLD.event_type
       OR NEW.created_at IS DISTINCT FROM OLD.created_at
       OR OLD.published_at IS NOT NULL THEN
        RAISE EXCEPTION 'outbox rows may only be marked published once'
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER collection_outbox_immutable
    BEFORE UPDATE OR DELETE ON collection_outbox
    FOR EACH ROW EXECUTE FUNCTION collection_outbox_guard();
