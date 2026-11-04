ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS discount_cents BIGINT NOT NULL DEFAULT 0
    CHECK (discount_cents >= 0 AND discount_cents <= subtotal_cents + shipping_cents);

-- Campaign codes are redeemable once per customer, while aggregate usage
-- counters remain available for campaign and coupon limits.
CREATE UNIQUE INDEX IF NOT EXISTS promotion_usages_coupon_user_unique
    ON promotion_usages (coupon_id, user_id)
    WHERE coupon_id IS NOT NULL;
