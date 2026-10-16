-- name: ListApprovedProducts :many
SELECT
    p.id,
    p.slug,
    b.name AS brand,
    p.name,
    p.description,
    c.slug AS category_slug,
    c.name AS category_name,
    pv.price_cents,
    COALESCE(pv.compare_at_cents, 0) AS compare_at_cents,
    (pv.compare_at_cents IS NOT NULL)::boolean AS has_compare_at,
    p.rating::float8 AS rating,
    p.review_count,
    p.delivery_label,
    COALESCE(NULLIF(p.image_url, ''), (SELECT pm.public_url FROM product_media pm WHERE pm.product_id = p.id AND pm.status = 'ready' AND pm.visibility = 'public' AND pm.content_type LIKE 'image/%' ORDER BY pm.id DESC LIMIT 1), '')::text AS image_url,
    p.swatches
    ,pv.id AS variant_id
    ,COALESCE(i.available_quantity, 0) AS available_quantity
FROM products p
JOIN categories c ON c.slug = p.category_slug
JOIN brands b ON b.slug = p.brand_slug
JOIN sellers s ON s.id = p.seller_id AND s.status = 'active'
JOIN LATERAL (
    SELECT variant.id, variant.price_cents, variant.compare_at_cents
    FROM product_variants variant
    LEFT JOIN inventory_stock candidate_stock ON candidate_stock.variant_id=variant.id
    WHERE variant.product_id=p.id AND variant.status='active'
    ORDER BY (COALESCE(candidate_stock.available_quantity, 0) > 0) DESC, variant.id
    LIMIT 1
) pv ON TRUE
LEFT JOIN inventory_stock i ON i.variant_id = pv.id
WHERE p.status = 'approved'
  AND (@category_slug::text = '' OR EXISTS(SELECT 1 FROM product_categories pc WHERE pc.product_id=p.id AND pc.category_slug = @category_slug::text))
ORDER BY p.featured_rank ASC, p.id ASC
LIMIT @result_limit;

-- name: GetApprovedProductBySlug :one
SELECT
    p.id,
    p.slug,
    b.name AS brand,
    p.name,
    p.description,
    c.slug AS category_slug,
    c.name AS category_name,
    pv.price_cents,
    COALESCE(pv.compare_at_cents, 0) AS compare_at_cents,
    (pv.compare_at_cents IS NOT NULL)::boolean AS has_compare_at,
    p.rating::float8 AS rating,
    p.review_count,
    p.delivery_label,
    COALESCE(NULLIF(p.image_url, ''), (SELECT pm.public_url FROM product_media pm WHERE pm.product_id = p.id AND pm.status = 'ready' AND pm.visibility = 'public' AND pm.content_type LIKE 'image/%' ORDER BY pm.id DESC LIMIT 1), '')::text AS image_url,
    p.swatches
    ,pv.id AS variant_id
    ,COALESCE(i.available_quantity, 0) AS available_quantity
FROM products p
JOIN categories c ON c.slug = p.category_slug
JOIN brands b ON b.slug = p.brand_slug
JOIN sellers s ON s.id = p.seller_id AND s.status = 'active'
JOIN LATERAL (
    SELECT variant.id, variant.price_cents, variant.compare_at_cents
    FROM product_variants variant
    LEFT JOIN inventory_stock candidate_stock ON candidate_stock.variant_id=variant.id
    WHERE variant.product_id=p.id AND variant.status='active'
    ORDER BY (COALESCE(candidate_stock.available_quantity, 0) > 0) DESC, variant.id
    LIMIT 1
) pv ON TRUE
LEFT JOIN inventory_stock i ON i.variant_id = pv.id
WHERE p.status = 'approved' AND p.slug = @slug;
