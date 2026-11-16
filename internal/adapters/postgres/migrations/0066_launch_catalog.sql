CREATE TABLE marketplace_launch_variants (
    variant_id bigint PRIMARY KEY REFERENCES product_variants(id) ON DELETE CASCADE
);

CREATE VIEW marketplace_variant_readiness AS
SELECT v.id AS variant_id, p.seller_id, p.name AS product, v.sku, s.display_name AS seller,
       v.shipping_weight_g, v.shipping_length_cm, v.shipping_width_cm, v.shipping_height_cm,
       COALESCE(sp.verified,false) AS pickup_ready,
       ip.seller_id IS NOT NULL AS invoice_ready,
       COALESCE(tp.variant_id IS NOT NULL AND (ip.registration<>'unregistered' OR tp.rate_basis_points=0),false) AS tax_ready,
       v.return_class<>'unclassified' AS classified,
       COALESCE(i.available_quantity,0) AS stock,
       lv.variant_id IS NOT NULL AS selected
FROM product_variants v
JOIN products p ON p.id=v.product_id AND p.status='approved'
JOIN sellers s ON s.id=p.seller_id AND s.status='active'
LEFT JOIN marketplace_shipping_profiles sp ON sp.seller_id=s.id
LEFT JOIN seller_invoice_profiles ip ON ip.seller_id=s.id
LEFT JOIN variant_tax_profiles tp ON tp.variant_id=v.id
LEFT JOIN inventory_stock i ON i.variant_id=v.id
LEFT JOIN marketplace_launch_variants lv ON lv.variant_id=v.id
WHERE v.status='active';

CREATE VIEW marketplace_ready_variants AS
SELECT variant_id,seller_id,stock,selected
FROM marketplace_variant_readiness
WHERE pickup_ready AND invoice_ready AND tax_ready AND classified
  AND shipping_weight_g BETWEEN 100 AND 50000
  AND shipping_length_cm BETWEEN 1 AND 200
  AND shipping_width_cm BETWEEN 1 AND 200
  AND shipping_height_cm BETWEEN 1 AND 200;
