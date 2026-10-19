DROP INDEX IF EXISTS idx_rooms_organizer_name;

ALTER TABLE rooms
    DROP COLUMN IF EXISTS organizer_pin_hash;

ALTER TABLE rooms
    DROP COLUMN IF EXISTS organizer_name;
