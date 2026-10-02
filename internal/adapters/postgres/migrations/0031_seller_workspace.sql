ALTER TABLE sellers
    ADD COLUMN IF NOT EXISTS low_stock_threshold INTEGER NOT NULL DEFAULT 5
    CHECK (low_stock_threshold BETWEEN 1 AND 100000);

ALTER TABLE product_variants
    ADD COLUMN IF NOT EXISTS cost_price_cents BIGINT
    CHECK (cost_price_cents IS NULL OR cost_price_cents >= 0);
