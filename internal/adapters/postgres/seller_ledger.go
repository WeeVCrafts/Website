package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func recordSellerSalesLedger(ctx context.Context, tx pgx.Tx, orderID int64) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(4856203)`); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		WITH lines AS (
			SELECT oi.id AS order_item_id, p.seller_id, oi.line_total_cents AS merchandise_cents,
			       COALESCE(oi.allocated_discount_cents,0) AS discount_cents,
			       oi.allocated_discount_cents IS NOT NULL AS allocation_saved,
			       rule.id AS commission_rule_id, COALESCE(rule.percentage_basis_points,0) AS basis_points
			FROM order_items oi
			JOIN product_variants pv ON pv.id=oi.variant_id
			JOIN products p ON p.id=pv.product_id
			LEFT JOIN LATERAL (
				SELECT id,percentage_basis_points FROM commission_rules cr
				WHERE cr.active AND (cr.category_slug IS NULL OR cr.category_slug=p.category_slug)
				ORDER BY (cr.category_slug IS NOT NULL) DESC,cr.id DESC LIMIT 1
			) rule ON TRUE
			WHERE oi.order_id=$1
		), earnings AS (
			SELECT *, merchandise_cents-discount_cents AS gross_cents,
			       FLOOR((merchandise_cents-discount_cents)::numeric*basis_points/10000)::bigint AS commission_cents
			FROM lines
		), snapshot AS (
			UPDATE orders o SET seller_finance_snapshot=jsonb_build_object(
				'version',1,'currency',o.currency,'shipping_cents',o.shipping_cents,
				'shipping_discount_cents',o.shipping_discount_cents,
				'allocation_status',CASE WHEN o.shipping_discount_cents IS NOT NULL AND totals.all_saved
					THEN 'saved' ELSE 'legacy_unallocated' END,
				'lines',totals.lines)
			FROM (SELECT jsonb_agg(to_jsonb(e) ORDER BY order_item_id) AS lines,
			             BOOL_AND(allocation_saved) AS all_saved FROM earnings e) totals
			WHERE o.id=$1 AND o.seller_finance_snapshot IS NULL AND totals.lines IS NOT NULL
			  AND NOT EXISTS (SELECT 1 FROM seller_ledger sl WHERE sl.order_id=o.id AND sl.entry_type='sale')
			RETURNING o.id,o.order_number,o.seller_finance_snapshot
		)
		INSERT INTO seller_ledger (seller_id,order_id,entry_type,gross_cents,commission_cents,net_cents,status,note)
		SELECT e.seller_id,s.id,'sale',SUM(e.gross_cents)::bigint,SUM(e.commission_cents)::bigint,
		       SUM(e.gross_cents-e.commission_cents)::bigint,'open','Order '||s.order_number
		FROM snapshot s CROSS JOIN LATERAL jsonb_to_recordset(s.seller_finance_snapshot->'lines')
		     AS e(seller_id bigint,gross_cents bigint,commission_cents bigint)
		GROUP BY e.seller_id,s.id,s.order_number
		ON CONFLICT (seller_id,order_id,entry_type) WHERE order_id IS NOT NULL DO NOTHING`, orderID)
	return err
}

func recordSellerRefundLedger(ctx context.Context, tx pgx.Tx, orderID int64) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(4856203)`); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO seller_ledger (seller_id,order_id,entry_type,gross_cents,commission_cents,net_cents,status,note)
		SELECT sl.seller_id,sl.order_id,'refund',-sl.gross_cents,-sl.commission_cents,-sl.net_cents,
		       'open','Refund for order '||o.order_number
		FROM seller_ledger sl JOIN orders o ON o.id=sl.order_id
		WHERE sl.order_id=$1 AND sl.entry_type='sale'
		ON CONFLICT (seller_id,order_id,entry_type) WHERE order_id IS NOT NULL DO NOTHING`, orderID)
	return err
}
