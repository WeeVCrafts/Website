-- NULL identifies historical orders whose discount allocation was not saved.
-- Do not infer historical coupon targeting from today's mutable promotions.
ALTER TABLE orders
    ADD COLUMN shipping_discount_cents BIGINT
        CHECK (shipping_discount_cents >= 0 AND shipping_discount_cents <= shipping_cents),
    ADD COLUMN seller_finance_snapshot JSONB;

ALTER TABLE order_items
    ADD COLUMN allocated_discount_cents BIGINT
        CHECK (allocated_discount_cents >= 0 AND allocated_discount_cents <= line_total_cents);
