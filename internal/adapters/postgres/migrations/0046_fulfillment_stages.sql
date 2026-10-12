ALTER TABLE seller_fulfillments
    DROP CONSTRAINT IF EXISTS seller_fulfillments_status_check;

ALTER TABLE seller_fulfillments
    ADD CONSTRAINT seller_fulfillments_status_check
    CHECK (status IN ('pending', 'processing', 'packing', 'shipped', 'out_for_delivery', 'delivered'));

ALTER TABLE seller_fulfillments
    ADD COLUMN IF NOT EXISTS revision BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS seller_fulfillments_revision_idx
    ON seller_fulfillments (order_id, seller_id, revision);
