CREATE TABLE IF NOT EXISTS rooms (
    id text primary key,
    title text NOT NULL,
    currency text NOT NULL,
    service_fee bigint NOT NULL DEFAULT 0,
    tip_amount bigint NOT NULL DEFAULT 0,
    discount bigint NOT NULL DEFAULT 0,
    total_amount bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now (),
    updated_at timestamptz NOT NULL DEFAULT now ()
);

CREATE TABLE IF NOT EXISTS participants (
    id text primary key,
    room_id text NOT NULL references rooms (id) ON DELETE CASCADE,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now ()
);

CREATE TABLE IF NOT EXISTS receipt_items (
    id text primary key,
    room_id text NOT NULL references rooms (id) ON DELETE CASCADE,
    name text NOT NULL,
    quantity integer NOT NULL DEFAULT 1,
    unit_price bigint not null default 0,
    total bigint not null,
    created_at timestamptz NOT NULL DEFAULT now ()
);

CREATE TABLE IF NOT EXISTS item_assignments (
    room_id text NOT NULL references rooms (id) ON DELETE CASCADE,
    item_id text NOT NULL references receipt_items (id) ON DELETE CASCADE,
    participant_id text NOT NULL references participants (id) ON DELETE CASCADE,
    weight bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now (),
    PRIMARY KEY (room_id, item_id, participant_id)
);

CREATE INDEX IF NOT EXISTS idx_participants_room_id ON participants (room_id);

CREATE INDEX IF NOT EXISTS idx_receipt_items_room_id ON receipt_items (room_id);

CREATE INDEX IF NOT EXISTS idx_item_assignments_room_id ON item_assignments (room_id);