-- +goose Up
ALTER TABLE messages
ADD COLUMN deleted_at TIMESTAMPTZ;

-- Preserve existing soft-deleted rows created by the old content=NULL behavior.
UPDATE messages
SET deleted_at = COALESCE(updated_at, now())
WHERE content IS NULL
  AND deleted_at IS NULL;

-- +goose Down
ALTER TABLE messages
DROP COLUMN IF EXISTS deleted_at;
