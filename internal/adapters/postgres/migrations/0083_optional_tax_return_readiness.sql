-- 0083_optional_tax_return_readiness.sql
-- Make tax profiles and return classifications optional for catalog/checkout readiness.
-- Unclassified return options default to 'textile_craft', and missing variant tax profiles
-- default to standard craft GST (5% for registered sellers, 0% for unregistered).

CREATE OR REPLACE VIEW marketplace_option_setup_facts AS
SELECT v.id AS variant_id,p.id AS product_id,p.seller_id,p.name AS product,v.sku,s.display_name AS seller,
    p.status AS product_status,parcel.shipping_weight_g,parcel.shipping_length_cm,parcel.shipping_width_cm,
    parcel.shipping_height_cm,COALESCE(sp.verified,false) AS pickup_ready,
    (ip.seller_id IS NOT NULL AND NOT ip.checkout_review_hold) AS invoice_ready,
    COALESCE(NOT COALESCE(tp.checkout_review_hold, false), true) AS tax_ready,
    true AS classified,
    COALESCE(i.available_quantity,0) AS stock,lv.variant_id IS NOT NULL AS selected,
    COALESCE(NULLIF(p.image_url,''),(SELECT pm.public_url FROM product_media pm WHERE pm.product_id=p.id AND pm.status='ready' AND pm.content_type LIKE 'image/%' ORDER BY pm.id DESC LIMIT 1),'') AS image_url,
    COALESCE(tp.variant_id IS NOT NULL, false) AS has_custom_tax,
    CASE WHEN v.return_class='unclassified' THEN 'textile_craft' ELSE v.return_class END AS return_class
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

CREATE OR REPLACE VIEW marketplace_product_checkout_summary AS
SELECT v.product_id,
    count(*) FILTER (WHERE o.fulfillment_ready AND o.live_allowed AND o.stock>0) AS ready_count,
    CASE
        WHEN bool_and(o.stock<=0) THEN 'Out of stock'
        WHEN bool_or(NOT o.pickup_ready) THEN 'Verify pickup location'
        WHEN bool_or(NOT o.invoice_ready) THEN 'Complete business details'
        WHEN bool_or(o.stock>0 AND NOT (o.shipping_weight_g BETWEEN 100 AND 50000
            AND o.shipping_length_cm BETWEEN 1 AND 200 AND o.shipping_width_cm BETWEEN 1 AND 200
            AND o.shipping_height_cm BETWEEN 1 AND 200)) THEN 'Add packed measurements'
        ELSE 'Awaiting marketplace launch'
    END AS checkout_readiness_reason
FROM product_variants v
JOIN marketplace_checkout_options o ON o.variant_id=v.id
GROUP BY v.product_id;

CREATE OR REPLACE FUNCTION marketplace_item_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  t_hsn text;
  t_rate integer;
BEGIN
  SELECT p.seller_id,
         CASE WHEN v.return_class='unclassified' THEN 'textile_craft' ELSE v.return_class END
  INTO NEW.seller_id,NEW.return_class
  FROM product_variants v
  JOIN products p ON p.id=v.product_id
  WHERE v.id=NEW.variant_id;

  SELECT t.hsn, t.rate_basis_points INTO t_hsn, t_rate
  FROM variant_tax_profiles t WHERE t.variant_id=NEW.variant_id;

  IF t_hsn IS NULL THEN
    SELECT CASE WHEN ip.registration='unregistered' THEN '' ELSE '9701' END,
           CASE WHEN ip.registration='unregistered' THEN 0 ELSE 500 END
    INTO t_hsn, t_rate
    FROM seller_invoice_profiles ip WHERE ip.seller_id=NEW.seller_id;
  END IF;

  NEW.tax_snapshot := jsonb_build_object(
    'hsn', COALESCE(t_hsn, '9701'),
    'rate_basis_points', COALESCE(t_rate, 500),
    'inclusive', true
  );
  RETURN NEW;
END $$;
