ALTER TABLE carts ADD COLUMN purpose TEXT NOT NULL DEFAULT 'shopping'
    CHECK (purpose IN ('shopping','buy_now'));
ALTER TABLE carts ADD COLUMN checkout_token TEXT;
ALTER TABLE carts ADD COLUMN intent_request_key TEXT;
ALTER TABLE carts ADD COLUMN checkout_order_id BIGINT REFERENCES orders(id) ON DELETE SET NULL;
ALTER TABLE carts ADD CONSTRAINT carts_buy_now_owner CHECK
    (purpose='shopping' OR (user_id IS NOT NULL AND guest_token_hash IS NULL AND checkout_token IS NOT NULL AND intent_request_key IS NOT NULL));
CREATE UNIQUE INDEX carts_checkout_token_idx ON carts(checkout_token) WHERE checkout_token IS NOT NULL;
CREATE UNIQUE INDEX carts_intent_request_idx ON carts(user_id,intent_request_key) WHERE purpose='buy_now';
DROP INDEX carts_active_user_idx;
CREATE UNIQUE INDEX carts_active_user_idx ON carts(user_id)
    WHERE status='active' AND user_id IS NOT NULL AND purpose='shopping';
