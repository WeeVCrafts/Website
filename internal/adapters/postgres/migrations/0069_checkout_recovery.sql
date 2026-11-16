CREATE TABLE checkout_recovery (
    cart_id bigint PRIMARY KEY REFERENCES carts(id) ON DELETE CASCADE,
    first_quoted_at timestamptz NOT NULL DEFAULT now(),
    last_quoted_at timestamptz NOT NULL DEFAULT now(),
    order_id bigint REFERENCES orders(id) ON DELETE SET NULL,
    notice_started_at timestamptz,
    notice_sent_at timestamptz
);
CREATE INDEX checkout_recovery_due ON checkout_recovery(last_quoted_at,cart_id)
    WHERE order_id IS NULL AND notice_sent_at IS NULL;
