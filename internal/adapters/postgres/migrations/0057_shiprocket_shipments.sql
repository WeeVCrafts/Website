CREATE TABLE shiprocket_shipments (
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    seller_id BIGINT NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    requested_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    environment TEXT NOT NULL CHECK (environment IN ('sandbox', 'live')),
    reference TEXT NOT NULL,
    pickup_location TEXT NOT NULL,
    weight_grams INTEGER NOT NULL CHECK (weight_grams > 0),
    length_cm INTEGER NOT NULL CHECK (length_cm > 0),
    width_cm INTEGER NOT NULL CHECK (width_cm > 0),
    height_cm INTEGER NOT NULL CHECK (height_cm > 0),
    provider_order_id BIGINT,
    provider_shipment_id BIGINT,
    pending_action TEXT NOT NULL DEFAULT '' CHECK (pending_action IN ('', 'create', 'assign', 'pickup', 'label')),
    provider_status TEXT NOT NULL DEFAULT '',
    awb TEXT NOT NULL DEFAULT '',
    courier TEXT NOT NULL DEFAULT '',
    pickup_reference TEXT NOT NULL DEFAULT '',
    label_url TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    next_check_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (order_id, seller_id, environment),
    UNIQUE (environment, reference),
    UNIQUE (environment, provider_order_id),
    UNIQUE (environment, provider_shipment_id)
);

CREATE INDEX shiprocket_shipments_due ON shiprocket_shipments(environment, next_check_at)
    WHERE provider_status NOT IN ('DELIVERED', 'CANCELED', 'CANCELLED', 'RTO DELIVERED');
