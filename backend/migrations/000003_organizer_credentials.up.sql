ALTER TABLE rooms
    ADD COLUMN IF NOT EXISTS organizer_name text NOT NULL DEFAULT '';

ALTER TABLE rooms
    ADD COLUMN IF NOT EXISTS organizer_pin_hash text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_rooms_organizer_name ON rooms (lower(organizer_name))
WHERE
    organizer_name <> '';
