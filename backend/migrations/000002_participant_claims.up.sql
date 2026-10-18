ALTER TABLE participants
    ADD COLUMN IF NOT EXISTS claimed boolean NOT NULL DEFAULT false;

UPDATE participants
SET claimed = true
WHERE access_token IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_participants_room_claimed ON participants (room_id, claimed);
