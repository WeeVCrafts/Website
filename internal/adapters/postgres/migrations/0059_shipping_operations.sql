ALTER TABLE product_variants ADD COLUMN shipping_weight_g integer NOT NULL DEFAULT 0,
    ADD COLUMN shipping_length_cm integer NOT NULL DEFAULT 0,
    ADD COLUMN shipping_width_cm integer NOT NULL DEFAULT 0,
    ADD COLUMN shipping_height_cm integer NOT NULL DEFAULT 0,
    ADD CHECK ((shipping_weight_g = 0 AND shipping_length_cm = 0 AND shipping_width_cm = 0 AND shipping_height_cm = 0) OR
    (shipping_weight_g BETWEEN 1 AND 50000 AND shipping_length_cm BETWEEN 1 AND 200 AND shipping_width_cm BETWEEN 1 AND 200 AND shipping_height_cm BETWEEN 1 AND 200));

CREATE TABLE order_delivery_decisions (
    order_id bigint NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    seller_id bigint NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    decision text NOT NULL CHECK (decision IN ('shiprocket','manual','manual_fallback')),
    estimate jsonb NOT NULL,
    PRIMARY KEY (order_id,seller_id)
);

CREATE TABLE delivery_exceptions (
    order_id bigint NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    seller_id bigint NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    environment text NOT NULL CHECK (environment IN ('sandbox','live')),
    kind text NOT NULL CHECK (kind IN ('checkout_fallback','pending_operation','document_failed','provider_exception','cancelled_parcel')),
    assignee_id bigint REFERENCES users(id) ON DELETE SET NULL,
    opened_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    resolved_by bigint REFERENCES users(id) ON DELETE SET NULL,
    resolution text NOT NULL DEFAULT '',
    PRIMARY KEY (order_id,seller_id,environment,kind)
);

ALTER TABLE shiprocket_shipments DROP CONSTRAINT shiprocket_shipments_pending_action_check,
    ADD CONSTRAINT shiprocket_shipments_pending_action_check
        CHECK (pending_action IN ('','create','assign','pickup','label','manifest','cancel'));

ALTER TABLE shiprocket_shipments ADD COLUMN pending_since timestamptz;
UPDATE shiprocket_shipments SET pending_since=updated_at WHERE pending_action<>'';
CREATE INDEX delivery_exceptions_open_idx ON delivery_exceptions(opened_at) WHERE resolved_at IS NULL;

INSERT INTO delivery_exceptions (order_id,seller_id,environment,kind,opened_at)
SELECT order_id,seller_id,environment,'pending_operation',min(pending_since)
FROM shiprocket_shipments WHERE pending_action<>''
GROUP BY order_id,seller_id,environment;
