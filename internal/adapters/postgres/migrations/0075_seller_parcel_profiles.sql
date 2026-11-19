CREATE TABLE seller_parcel_profiles (
    id bigserial PRIMARY KEY,
    seller_id bigint NOT NULL REFERENCES sellers(id),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    weight_grams integer NOT NULL CHECK (weight_grams BETWEEN 100 AND 50000),
    length_cm integer NOT NULL CHECK (length_cm BETWEEN 1 AND 200),
    width_cm integer NOT NULL CHECK (width_cm BETWEEN 1 AND 200),
    height_cm integer NOT NULL CHECK (height_cm BETWEEN 1 AND 200),
    UNIQUE (seller_id, name),
    UNIQUE (seller_id, id)
);

CREATE TABLE seller_parcel_defaults (
    seller_id bigint PRIMARY KEY REFERENCES sellers(id),
    profile_id bigint NOT NULL,
    FOREIGN KEY (seller_id, profile_id) REFERENCES seller_parcel_profiles(seller_id, id)
);

ALTER TABLE product_variants
    ADD COLUMN parcel_profile_id bigint REFERENCES seller_parcel_profiles(id),
    ADD COLUMN use_seller_parcel_default boolean NOT NULL DEFAULT false,
    ADD CHECK (parcel_profile_id IS NULL OR NOT use_seller_parcel_default);

CREATE INDEX product_variants_seller_parcel_default ON product_variants(product_id)
    WHERE use_seller_parcel_default;

CREATE VIEW marketplace_variant_parcels AS
SELECT v.id AS variant_id, p.seller_id,
    CASE WHEN m.has_override THEN v.shipping_weight_g ELSE COALESCE(pp.weight_grams, v.shipping_weight_g) END AS shipping_weight_g,
    CASE WHEN m.has_override THEN v.shipping_length_cm ELSE COALESCE(pp.length_cm, v.shipping_length_cm) END AS shipping_length_cm,
    CASE WHEN m.has_override THEN v.shipping_width_cm ELSE COALESCE(pp.width_cm, v.shipping_width_cm) END AS shipping_width_cm,
    CASE WHEN m.has_override THEN v.shipping_height_cm ELSE COALESCE(pp.height_cm, v.shipping_height_cm) END AS shipping_height_cm,
    m.has_override,
    CASE
        WHEN m.has_override THEN 'Measured override'
        WHEN v.parcel_profile_id IS NOT NULL AND pp.id IS NOT NULL THEN pp.name
        WHEN pp.id IS NOT NULL THEN pp.name || ' (seller default)'
        ELSE 'Missing measurements'
    END AS source
FROM product_variants v
JOIN products p ON p.id = v.product_id
LEFT JOIN seller_parcel_defaults d ON d.seller_id = p.seller_id AND v.use_seller_parcel_default
LEFT JOIN seller_parcel_profiles pp ON pp.seller_id = p.seller_id
    AND pp.id = COALESCE(v.parcel_profile_id, d.profile_id)
CROSS JOIN LATERAL (
    SELECT v.shipping_weight_g BETWEEN 100 AND 50000
       AND v.shipping_length_cm BETWEEN 1 AND 200
       AND v.shipping_width_cm BETWEEN 1 AND 200
       AND v.shipping_height_cm BETWEEN 1 AND 200 AS has_override
) m;

CREATE OR REPLACE VIEW marketplace_variant_readiness AS
SELECT v.id AS variant_id,p.seller_id,p.name AS product,v.sku,s.display_name AS seller,
    parcel.shipping_weight_g,parcel.shipping_length_cm,parcel.shipping_width_cm,parcel.shipping_height_cm,
    COALESCE(sp.verified,false) AS pickup_ready,
    (ip.seller_id IS NOT NULL AND NOT ip.checkout_review_hold) AS invoice_ready,
    COALESCE(tp.variant_id IS NOT NULL AND NOT tp.checkout_review_hold AND (ip.registration<>'unregistered' OR tp.rate_basis_points=0),false) AS tax_ready,
    v.return_class<>'unclassified' AS classified,COALESCE(i.available_quantity,0) AS stock,lv.variant_id IS NOT NULL AS selected
FROM product_variants v
JOIN products p ON p.id=v.product_id AND p.status='approved'
JOIN sellers s ON s.id=p.seller_id AND s.status='active'
JOIN marketplace_variant_parcels parcel ON parcel.variant_id=v.id
LEFT JOIN marketplace_shipping_profiles sp ON sp.seller_id=s.id
LEFT JOIN seller_invoice_profiles ip ON ip.seller_id=s.id
LEFT JOIN variant_tax_profiles tp ON tp.variant_id=v.id
LEFT JOIN inventory_stock i ON i.variant_id=v.id
LEFT JOIN marketplace_launch_variants lv ON lv.variant_id=v.id
WHERE v.status='active';
