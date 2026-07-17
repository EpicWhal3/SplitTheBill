CREATE TABLE IF NOT EXISTS rooms (
    id text PRIMARY KEY,
    title text NOT NULL,
    currency text NOT NULL,

    service_fee bigint NOT NULL DEFAULT 0
        CHECK (service_fee >= 0),

    tip_amount bigint NOT NULL DEFAULT 0
        CHECK (tip_amount >= 0),

    discount bigint NOT NULL DEFAULT 0
        CHECK (discount >= 0),

    expected_total bigint NOT NULL DEFAULT 0
        CHECK (expected_total >= 0),

    admin_token text NOT NULL UNIQUE,

    payer_participant_id text,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS participants (
    id text PRIMARY KEY,

    room_id text NOT NULL
        REFERENCES rooms(id)
        ON DELETE CASCADE,

    name text NOT NULL,

    access_token text UNIQUE,

    created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE rooms
    ADD CONSTRAINT rooms_payer_participant_fk
    FOREIGN KEY (payer_participant_id)
    REFERENCES participants(id)
    ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS receipt_items (
    id text PRIMARY KEY,

    room_id text NOT NULL
        REFERENCES rooms(id)
        ON DELETE CASCADE,

    name text NOT NULL,

    quantity integer NOT NULL DEFAULT 1
        CHECK (quantity > 0),

    unit_price bigint NOT NULL
        CHECK (unit_price > 0),

    total bigint NOT NULL
        CHECK (total > 0),

    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS item_assignments (
    room_id text NOT NULL
        REFERENCES rooms(id)
        ON DELETE CASCADE,

    item_id text NOT NULL
        REFERENCES receipt_items(id)
        ON DELETE CASCADE,

    participant_id text NOT NULL
        REFERENCES participants(id)
        ON DELETE CASCADE,

    weight bigint NOT NULL DEFAULT 1
        CHECK (weight > 0),

    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (
        room_id,
        item_id,
        participant_id
    )
);

CREATE INDEX IF NOT EXISTS idx_participants_room_id
    ON participants(room_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_participants_access_token
    ON participants(access_token)
    WHERE access_token IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_receipt_items_room_id
    ON receipt_items(room_id);

CREATE INDEX IF NOT EXISTS idx_item_assignments_room_id
    ON item_assignments(room_id);

CREATE INDEX IF NOT EXISTS idx_item_assignments_item_id
    ON item_assignments(item_id);

CREATE INDEX IF NOT EXISTS idx_item_assignments_participant_id
    ON item_assignments(participant_id);

CREATE INDEX IF NOT EXISTS idx_rooms_payer_participant_id
    ON rooms(payer_participant_id);