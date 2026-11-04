package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

// Save allocations in the placement transaction while the promotion is locked.
// Cumulative integer rounding allocates every paisa exactly once, in item-ID order.
func snapshotOrderDiscounts(ctx context.Context, tx pgx.Tx, orderID, promotionID int64) error {
	_, err := tx.Exec(ctx, `
		WITH policy AS (
			SELECT o.id, o.discount_cents,
			       COALESCE(p.discount_type='free_shipping',false) AS free_shipping,
			       COALESCE(p.targeting_mode,'all') AS targeting_mode
			FROM orders o LEFT JOIN promotions p ON p.id=$2 WHERE o.id=$1
		), shipping AS (
			UPDATE orders o SET shipping_discount_cents=CASE WHEN p.free_shipping THEN p.discount_cents ELSE 0 END
			FROM policy p WHERE o.id=p.id
		), eligible AS (
			SELECT oi.id, oi.line_total_cents, p.discount_cents, p.free_shipping,
			       CASE WHEN p.targeting_mode='all' OR EXISTS (
				SELECT 1 FROM promotion_products pp JOIN product_variants pv ON pv.product_id=pp.product_id
				WHERE pp.promotion_id=$2 AND pv.id=oi.variant_id
			       ) THEN oi.line_total_cents ELSE 0 END AS weight
			FROM order_items oi JOIN policy p ON p.id=oi.order_id
		), weights AS (
			SELECT *, SUM(weight) OVER () AS total_weight,
			       SUM(weight) OVER (ORDER BY id ROWS UNBOUNDED PRECEDING) AS cumulative_weight
			FROM eligible
		)
		UPDATE order_items oi SET allocated_discount_cents=CASE
			WHEN w.free_shipping OR w.discount_cents=0 THEN 0
			ELSE (FLOOR(w.discount_cents::numeric*w.cumulative_weight/NULLIF(w.total_weight,0))
			     -FLOOR(w.discount_cents::numeric*(w.cumulative_weight-w.weight)/NULLIF(w.total_weight,0)))::bigint END
		FROM weights w WHERE oi.id=w.id`, orderID, promotionID)
	if err != nil {
		return err
	}
	var reconciles bool
	err = tx.QueryRow(ctx, `SELECT o.shipping_discount_cents+SUM(oi.allocated_discount_cents)=o.discount_cents
		AND BOOL_AND(oi.allocated_discount_cents IS NOT NULL)
		FROM orders o JOIN order_items oi ON oi.order_id=o.id WHERE o.id=$1 GROUP BY o.id`, orderID).Scan(&reconciles)
	if err != nil {
		return err
	}
	if !reconciles {
		return ports.ErrInvalidState
	}
	return nil
}
