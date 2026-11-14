-- name: InsertEvent :one
INSERT INTO webhook_events (id, merchant_id, type, object_id, payload, created_at)
VALUES (@id, @merchant_id, @type, @object_id, @payload, @created_at)
ON CONFLICT (type, object_id) DO NOTHING
RETURNING id;

-- name: GetEventByObject :one
SELECT id, merchant_id, type, object_id, payload, created_at
  FROM webhook_events
 WHERE type = @type AND object_id = @object_id;

-- name: GetEvent :one
SELECT id, merchant_id, type, object_id, payload, created_at
  FROM webhook_events
 WHERE id = @id AND merchant_id = @merchant_id;
