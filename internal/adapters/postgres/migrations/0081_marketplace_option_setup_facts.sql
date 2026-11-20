-- Compute seller setup facts once for both draft and published options.
-- The existing launch view still exposes published, active options only.
CREATE VIEW marketplace_option_setup_facts AS
SELECT v.id AS variant_id,p.id AS product_id,p.seller_id,p.name AS product,v.sku,s.display_name AS seller,
    p.status AS product_status,parcel.shipping_weight_g,parcel.shipping_length_cm,parcel.shipping_width_cm,
    parcel.shipping_height_cm,COALESCE(sp.verified,false) AS pickup_ready,
    (ip.seller_id IS NOT NULL AND NOT ip.checkout_review_hold) AS invoice_ready,
    COALESCE(tp.variant_id IS NOT NULL AND NOT tp.checkout_review_hold
        AND (ip.registration<>'unregistered' OR tp.rate_basis_points=0),false) AS tax_ready,
    v.return_class<>'unclassified' AS classified,
    COALESCE(i.available_quantity,0) AS stock,lv.variant_id IS NOT NULL AS selected
FROM product_variants v
JOIN products p ON p.id=v.product_id AND p.status<>'archived'
JOIN sellers s ON s.id=p.seller_id AND s.status='active'
JOIN marketplace_variant_parcels parcel ON parcel.variant_id=v.id
LEFT JOIN marketplace_shipping_profiles sp ON sp.seller_id=s.id
LEFT JOIN seller_invoice_profiles ip ON ip.seller_id=s.id
LEFT JOIN variant_tax_profiles tp ON tp.variant_id=v.id
LEFT JOIN inventory_stock i ON i.variant_id=v.id
LEFT JOIN marketplace_launch_variants lv ON lv.variant_id=v.id
WHERE v.status='active';

CREATE OR REPLACE VIEW marketplace_variant_readiness AS
SELECT variant_id,seller_id,product,sku,seller,shipping_weight_g,shipping_length_cm,shipping_width_cm,
    shipping_height_cm,pickup_ready,invoice_ready,tax_ready,classified,stock,selected
FROM marketplace_option_setup_facts
WHERE product_status='approved';
