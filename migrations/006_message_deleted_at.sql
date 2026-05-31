-- +goose Up
ALTER TABLE messages
ADD COLUMN deleted_at TIMESTAMPTZ;

-- Preserve existing soft-deleted rows created by the old content=NULL behavior.
-- Attachment messages are also persisted with NULL content but are NOT deleted,
-- so they must be excluded or the migration would hide valid files as deleted.
-- (Soft delete removes a message's attachments, so a genuinely deleted row has
-- none.) Content-less, attachment-less rows are treated as the old deletes.
UPDATE messages
SET deleted_at = COALESCE(updated_at, now())
WHERE content IS NULL
  AND deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM attachments a WHERE a.message_id = messages.id
  );

-- +goose Down
ALTER TABLE messages
DROP COLUMN IF EXISTS deleted_at;
