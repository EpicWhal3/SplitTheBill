ALTER TABLE receipt_items
    DROP CONSTRAINT IF EXISTS receipt_items_unit_price_check;

ALTER TABLE receipt_items
    DROP CONSTRAINT IF EXISTS receipt_items_total_check;

ALTER TABLE receipt_items
    ADD CONSTRAINT receipt_items_unit_price_check
    CHECK (unit_price >= 0);

ALTER TABLE receipt_items
    ADD CONSTRAINT receipt_items_total_check
    CHECK (total >= 0);
