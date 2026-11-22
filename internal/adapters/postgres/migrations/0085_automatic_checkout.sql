ALTER TABLE marketplace_policy_settings ADD COLUMN checkout_paused boolean NOT NULL DEFAULT false;

-- The old default was off before setup. Preserve only an actual operator pause.
UPDATE marketplace_policy_settings SET checkout_paused=true
WHERE id AND shipping_mode='off' AND EXISTS (
    SELECT 1 FROM audit_logs WHERE action='marketplace.pause_shipping'
);

CREATE TABLE marketplace_checkout_health (
    id boolean PRIMARY KEY DEFAULT true CHECK (id),
    checked_at timestamptz NOT NULL,
    valid_until timestamptz NOT NULL,
    blockers text[] NOT NULL,
    settings_key text NOT NULL DEFAULT '',
    pickup_postcodes jsonb NOT NULL CHECK (jsonb_typeof(pickup_postcodes)='object')
);

CREATE TABLE marketplace_checkout_issue (
    id bigint GENERATED ALWAYS AS IDENTITY UNIQUE,
    owner_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code text NOT NULL,
    PRIMARY KEY (owner_id,code)
);

CREATE TABLE marketplace_checkout_wait (
    cart_id bigint PRIMARY KEY REFERENCES carts(id) ON DELETE CASCADE,
    requested_at timestamptz NOT NULL DEFAULT now()
);

CREATE FUNCTION marketplace_fulfillment_ready(pickup boolean, invoice boolean, tax boolean,
                                              classified boolean, grams integer, length integer,
                                              width integer, height integer)
RETURNS boolean LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT pickup AND invoice AND tax AND classified AND grams BETWEEN 100 AND 50000
        AND length BETWEEN 1 AND 200 AND width BETWEEN 1 AND 200 AND height BETWEEN 1 AND 200;
$$;

CREATE OR REPLACE VIEW marketplace_ready_variants AS
SELECT variant_id,seller_id,stock,selected FROM marketplace_variant_readiness
WHERE marketplace_fulfillment_ready(pickup_ready,invoice_ready,tax_ready,classified,
    shipping_weight_g,shipping_length_cm,shipping_width_cm,shipping_height_cm);

CREATE OR REPLACE VIEW marketplace_checkout_options AS
SELECT r.*, marketplace_fulfillment_ready(r.pickup_ready,r.invoice_ready,r.tax_ready,r.classified,
    r.shipping_weight_g,r.shipping_length_cm,r.shipping_width_cm,r.shipping_height_cm) AS fulfillment_ready,
    CASE WHEN h.id THEN
        m.published_at IS NOT NULL AND NOT m.checkout_paused
        AND h.valid_until>statement_timestamp() AND cardinality(h.blockers)=0
        AND COALESCE(h.pickup_postcodes->>sp.pickup_location=sp.postal_code,false)
    ELSE COALESCE(m.published_at IS NOT NULL AND m.shipping_mode='live'
        AND (m.live_catalog_scope='ready'
            OR (m.live_catalog_scope='seller' AND m.live_catalog_seller_id=r.seller_id)
            OR (m.live_catalog_scope='limited' AND r.selected)),false)
    END AS live_allowed,
    COALESCE(h.id IS NULL AND m.published_at IS NOT NULL AND m.shipping_mode='pilot'
        AND m.pilot_variant_id=r.variant_id AND m.pilot_order_id IS NULL,false) AS pilot_allowed,
    m.pilot_user_id,
    h.id IS NOT NULL AS automatic,
    CASE WHEN h.id IS NULL THEN ''
        WHEN m.checkout_paused THEN 'paused'
        WHEN m.published_at IS NULL THEN 'policy'
        WHEN h.valid_until<=statement_timestamp() THEN 'stale'
        WHEN cardinality(h.blockers)>0 THEN h.blockers[1]
        WHEN NOT COALESCE(h.pickup_postcodes->>sp.pickup_location=sp.postal_code,false) THEN 'pickup'
        ELSE '' END AS live_blocker
FROM marketplace_variant_readiness r
LEFT JOIN marketplace_policy_settings m ON m.id
LEFT JOIN marketplace_checkout_health h ON h.id
LEFT JOIN marketplace_shipping_profiles sp ON sp.seller_id=r.seller_id;

CREATE OR REPLACE VIEW marketplace_product_checkout_summary AS
SELECT v.product_id,
    count(*) FILTER (WHERE o.fulfillment_ready AND o.live_allowed AND o.stock>0) AS ready_count,
    CASE
        WHEN bool_and(o.stock<=0) THEN 'Out of stock'
        WHEN bool_or(NOT o.invoice_ready) THEN 'Complete business details'
        WHEN bool_or(o.stock>0 AND NOT (o.shipping_weight_g BETWEEN 100 AND 50000
            AND o.shipping_length_cm BETWEEN 1 AND 200 AND o.shipping_width_cm BETWEEN 1 AND 200
            AND o.shipping_height_cm BETWEEN 1 AND 200)) THEN 'Add packed measurements'
        WHEN bool_or(o.stock>0 AND NOT (o.tax_ready AND o.classified)) THEN 'Review tax and return details'
        WHEN bool_or(NOT o.pickup_ready OR o.live_blocker='pickup') THEN 'Waiting for WeevCrafts: pickup verification'
        WHEN bool_or(o.automatic) THEN 'Waiting for WeevCrafts: checkout verification'
        ELSE 'Awaiting marketplace launch'
    END AS next_step
FROM product_variants v
JOIN marketplace_checkout_options o ON o.variant_id=v.id
GROUP BY v.product_id;
