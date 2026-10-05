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
		INSERT INTO seller_ledger (seller_id,order_id,entry_type,gross_cents,commission_cents,net_cents,status,note)
		SELECT totals.seller_id,totals.order_id,'sale',totals.gross_cents,totals.commission_cents,
		       totals.gross_cents-totals.commission_cents,'open','Order '||o.order_number
		FROM (
			SELECT p.seller_id,oi.order_id,SUM(oi.line_total_cents)::BIGINT AS gross_cents,
			       SUM((oi.line_total_cents*COALESCE(rule.percentage_basis_points,0)/10000)::BIGINT)::BIGINT AS commission_cents
			FROM order_items oi
			JOIN product_variants pv ON pv.id=oi.variant_id
			JOIN products p ON p.id=pv.product_id
			LEFT JOIN LATERAL (
				SELECT percentage_basis_points FROM commission_rules cr
				WHERE cr.active AND (cr.category_slug IS NULL OR cr.category_slug=p.category_slug)
				ORDER BY (cr.category_slug IS NOT NULL) DESC,cr.id DESC LIMIT 1
			) rule ON TRUE
			WHERE oi.order_id=$1
			GROUP BY p.seller_id,oi.order_id
		) totals
		JOIN orders o ON o.id=totals.order_id
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
