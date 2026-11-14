-- name: InsertCollectionOutbox :exec
INSERT INTO collection_outbox (collection_id, event_type, created_at)
VALUES (@collection_id, @event_type, @created_at)
ON CONFLICT (collection_id, event_type) DO NOTHING;

-- name: ClaimCollectionOutbox :many
SELECT id, collection_id, event_type, created_at
  FROM collection_outbox
 WHERE published_at IS NULL
 ORDER BY id
 LIMIT @batch_size
   FOR UPDATE SKIP LOCKED;

-- name: MarkCollectionOutboxPublished :exec
UPDATE collection_outbox
   SET published_at = @published_at
 WHERE id = ANY(@ids::bigint[])
   AND published_at IS NULL;
