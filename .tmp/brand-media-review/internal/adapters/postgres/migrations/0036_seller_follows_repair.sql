CREATE TABLE IF NOT EXISTS seller_follows (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    seller_id BIGINT NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, seller_id)
);

CREATE INDEX IF NOT EXISTS seller_follows_seller_idx
    ON seller_follows (seller_id, created_at DESC);
