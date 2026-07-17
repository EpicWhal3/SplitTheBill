DROP TABLE IF EXISTS item_assignments;
DROP TABLE IF EXISTS receipt_items;

ALTER TABLE rooms
    DROP CONSTRAINT IF EXISTS rooms_payer_participant_fk;

DROP TABLE IF EXISTS participants;
DROP TABLE IF EXISTS rooms;