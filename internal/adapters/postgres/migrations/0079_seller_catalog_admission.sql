ALTER TABLE sellers ADD COLUMN catalog_admitted boolean NOT NULL DEFAULT false;

UPDATE sellers s SET catalog_admitted=true WHERE EXISTS (
    SELECT 1 FROM products p WHERE p.seller_id=s.id AND (
        p.status='approved' OR EXISTS (
            SELECT 1 FROM audit_logs a WHERE a.resource_type='product' AND a.resource_id=p.id::text
                AND a.action='catalog.product_reviewed' AND a.metadata->>'approved'='true'
        )
    )
);

CREATE FUNCTION seller_catalog_can_publish(target_seller bigint) RETURNS boolean
LANGUAGE sql STABLE AS $$
    SELECT EXISTS (
        SELECT 1 FROM sellers s WHERE s.id=target_seller AND s.status='active' AND (
            s.catalog_admitted OR EXISTS (SELECT 1 FROM products p WHERE p.seller_id=s.id AND p.status='approved')
        )
    );
$$;
