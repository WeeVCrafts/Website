ALTER TABLE marketplace_policy_settings ADD COLUMN live_catalog_scope text NOT NULL DEFAULT 'limited'
    CHECK (live_catalog_scope IN ('limited', 'seller', 'ready')),
    ADD COLUMN live_catalog_seller_id bigint REFERENCES sellers(id),
    ADD CHECK ((live_catalog_scope='seller') = (live_catalog_seller_id IS NOT NULL));

-- Keep the limited list intact so an operator can restore the limited launch.
CREATE VIEW marketplace_checkout_options AS
SELECT r.*, v.variant_id IS NOT NULL AS fulfillment_ready,
    COALESCE(m.published_at IS NOT NULL AND m.shipping_mode='live'
        AND (m.live_catalog_scope='ready'
            OR (m.live_catalog_scope='seller' AND m.live_catalog_seller_id=r.seller_id)
            OR (m.live_catalog_scope='limited' AND r.selected)), false) AS live_allowed,
    COALESCE(m.published_at IS NOT NULL AND m.shipping_mode='pilot'
        AND m.pilot_variant_id=r.variant_id AND m.pilot_order_id IS NULL, false) AS pilot_allowed,
    m.pilot_user_id
FROM marketplace_variant_readiness r
LEFT JOIN marketplace_ready_variants v ON v.variant_id=r.variant_id
LEFT JOIN marketplace_policy_settings m ON m.id;

CREATE FUNCTION marketplace_option_can_checkout(option_id bigint, customer_id bigint, units integer)
RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT EXISTS (
        SELECT 1 FROM marketplace_checkout_options o
        WHERE o.variant_id=option_id AND o.fulfillment_ready AND units>0 AND o.stock>=units
          AND (o.live_allowed OR (o.pilot_allowed AND o.pilot_user_id=customer_id AND units=1))
    );
$$;

CREATE VIEW marketplace_product_checkout_summary AS
SELECT v.product_id,
    count(*) FILTER (WHERE o.fulfillment_ready AND o.live_allowed AND o.stock>0) AS ready_count,
    CASE
        WHEN bool_and(o.stock<=0) THEN 'Out of stock'
        WHEN bool_or(NOT o.pickup_ready) THEN 'Verify pickup location'
        WHEN bool_or(NOT o.invoice_ready) THEN 'Complete business details'
        WHEN bool_or(o.stock>0 AND NOT (o.shipping_weight_g BETWEEN 100 AND 50000
            AND o.shipping_length_cm BETWEEN 1 AND 200 AND o.shipping_width_cm BETWEEN 1 AND 200
            AND o.shipping_height_cm BETWEEN 1 AND 200)) THEN 'Add packed measurements'
        WHEN bool_or(o.stock>0 AND NOT (o.tax_ready AND o.classified)) THEN 'Complete tax and return details'
        ELSE 'Awaiting marketplace launch'
    END AS next_step
FROM marketplace_checkout_options o
JOIN product_variants v ON v.id=o.variant_id
GROUP BY v.product_id;
