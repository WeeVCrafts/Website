ALTER TABLE seller_ledger
    ADD COLUMN IF NOT EXISTS settlement_id BIGINT REFERENCES seller_settlements(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS seller_ledger_order_entry_unique
    ON seller_ledger (seller_id, order_id, entry_type)
    WHERE order_id IS NOT NULL;

-- Remove non-order-linked demo balances. They were presentation fixtures, not
-- amounts earned by a seller, and must not enter real order reconciliation.
DELETE FROM seller_ledger WHERE note = 'Demo settlement entry';
DELETE FROM seller_settlements WHERE reference LIKE 'DEMO-SETTLEMENT-%';

-- Use the existing marketplace default commission policy if no policy is
-- configured yet. The rate is expressed in basis points (12.00%).
INSERT INTO commission_rules (name, category_slug, percentage_basis_points, active)
VALUES ('Default marketplace commission', NULL, 1200, TRUE)
ON CONFLICT (name) DO NOTHING;

-- Backfill historical paid orders once; future captures use the same ledger
-- projection in the payment transaction.
INSERT INTO seller_ledger (seller_id, order_id, entry_type, gross_cents, commission_cents, net_cents, status, note)
SELECT x.seller_id, x.order_id, 'sale', x.gross_cents, x.commission_cents,
       x.gross_cents - x.commission_cents,
       CASE WHEN o.status = 'refunded' THEN 'reversed' ELSE 'open' END,
       'Order ' || o.order_number
FROM (
    SELECT p.seller_id, oi.order_id, SUM(oi.line_total_cents)::BIGINT AS gross_cents,
           SUM((oi.line_total_cents * COALESCE(rule.percentage_basis_points,0) / 10000)::BIGINT)::BIGINT AS commission_cents
    FROM order_items oi
    JOIN product_variants pv ON pv.id=oi.variant_id
    JOIN products p ON p.id=pv.product_id
    LEFT JOIN LATERAL (
        SELECT percentage_basis_points FROM commission_rules cr
        WHERE cr.active AND (cr.category_slug IS NULL OR cr.category_slug=p.category_slug)
        ORDER BY (cr.category_slug IS NOT NULL) DESC, cr.id DESC LIMIT 1
    ) rule ON TRUE
    GROUP BY p.seller_id,oi.order_id
) x
JOIN orders o ON o.id=x.order_id
WHERE o.status IN ('paid','processing','shipped','delivered','refunded')
ON CONFLICT (seller_id,order_id,entry_type) WHERE order_id IS NOT NULL DO NOTHING;

INSERT INTO seller_ledger (seller_id, order_id, entry_type, gross_cents, commission_cents, net_cents, status, note)
SELECT seller_id, order_id, 'refund', -gross_cents, -commission_cents, -net_cents,
       'open', 'Refund for order ' || order_number
FROM seller_ledger sl JOIN orders o ON o.id=sl.order_id
WHERE sl.entry_type='sale' AND o.status='refunded'
ON CONFLICT (seller_id,order_id,entry_type) WHERE order_id IS NOT NULL DO NOTHING;
