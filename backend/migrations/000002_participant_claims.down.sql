DROP INDEX IF EXISTS idx_participants_room_claimed;

ALTER TABLE participants DROP COLUMN IF EXISTS claimed;
